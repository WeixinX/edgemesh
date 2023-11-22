package tunnel

import (
	"context"
	"fmt"
	"time"

	"github.com/fsnotify/fsnotify"
	ipfslog "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-kad-dht/dual"
	p2phost "github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/pnet"
	"github.com/libp2p/go-libp2p/core/routing"
	"github.com/libp2p/go-libp2p/p2p/host/autorelay"
	rcmgr "github.com/libp2p/go-libp2p/p2p/host/resource-manager"
	"github.com/libp2p/go-libp2p/p2p/host/resource-manager/obs"
	"github.com/libp2p/go-libp2p/p2p/net/connmgr"
	relayv2 "github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/relay"
	"github.com/libp2p/go-libp2p/p2p/protocol/holepunch"
	"github.com/libp2p/go-libp2p/p2p/protocol/identify"
	ma "github.com/multiformats/go-multiaddr"
	"k8s.io/klog/v2"

	"github.com/kubeedge/beehive/pkg/core"
	"github.com/kubeedge/edgemesh/pkg/apis/config/defaults"
	"github.com/kubeedge/edgemesh/pkg/apis/config/v1alpha1"
)

// Agent expose the tunnel ability.  TODO convert var to func
var Agent *EdgeTunnel

// EdgeTunnel is used for solving cross subset communication
type EdgeTunnel struct {
	Config           *v1alpha1.EdgeTunnelConfig
	p2pHost          p2phost.Host       // libp2p host
	hostCtx          context.Context    // ctx governs the lifetime of the libp2p host
	nodePeerMap      map[string]peer.ID // map of Kubernetes node name and peer.ID
	mdnsPeerChan     chan peer.AddrInfo
	dhtPeerChan      <-chan peer.AddrInfo
	isRelay          bool
	relayMap         RelayMap
	relayService     *relayv2.Relay
	holepunchService *holepunch.Service
	stopCh           chan struct{}
	cfgWatcher       *fsnotify.Watcher
}

// Name of EdgeTunnel
func (t *EdgeTunnel) Name() string {
	return defaults.EdgeTunnelModuleName
}

// Group of EdgeTunnel
func (t *EdgeTunnel) Group() string {
	return defaults.EdgeTunnelModuleName
}

// Enable indicates whether enable this module
func (t *EdgeTunnel) Enable() bool {
	return t.Config.Enable
}

// Start EdgeTunnel
func (t *EdgeTunnel) Start() {
	t.Run()
}

func (t *EdgeTunnel) Shutdown() {
	close(t.stopCh)
}

// Register edgetunnel to beehive modules
func Register(c *v1alpha1.EdgeTunnelConfig) error {
	agent, err := newEdgeTunnel(c)
	if err != nil {
		return fmt.Errorf("register module EdgeTunnel error: %v", err)
	}
	core.Register(agent)
	return nil
}

func newEdgeTunnel(c *v1alpha1.EdgeTunnelConfig) (*EdgeTunnel, error) {
	if !c.Enable {
		return &EdgeTunnel{Config: c}, nil
	}

	if c.EnableIpfsLog {
		ipfslog.SetAllLoggers(ipfslog.LevelDebug)
	}

	ctx := context.Background()
	opts := make([]libp2p.Option, 0) // libp2p options
	peerSource := make(chan peer.AddrInfo, c.MaxCandidates)

	privKey, err := GenerateKeyPairWithString(c.NodeName)
	if err != nil {
		return nil, fmt.Errorf("failed to generate private key: %w", err)
	}

	connMgr, err := connmgr.NewConnManager(
		100, // LowWater
		400, // HighWater,
		connmgr.WithGracePeriod(time.Minute))
	if err != nil {
		return nil, fmt.Errorf("failed to new conn manager: %w", err)
	}

	// agent 监听 20006 端口，gateway 监听 20007 端口
	listenAddr, err := generateListenAddr(c)
	if err != nil {
		return nil, fmt.Errorf("failed to generate listenAddr: %w", err)
	}

	// If this host is a relay node, we need to add its advertiseAddress
	relayMap := GenerateRelayMap(c.RelayNodes, c.Transport, c.ListenPort)
	myInfo, isRelay := relayMap[c.NodeName]
	if isRelay && c.Mode == defaults.ServerClientMode {
		opts = append(opts, libp2p.AddrsFactory(func(maddrs []ma.Multiaddr) []ma.Multiaddr {
			maddrs = append(maddrs, myInfo.Addrs...)
			return maddrs
		}))
	}

	// If the relayMap does not contain any public IP, NATService will not be able to assist this non-relay node to
	// identify its own network(public, private or unknown), so it needs to configure libp2p.ForceReachabilityPrivate()
	if !isRelay && !relayMap.ContainsPublicIP() {
		klog.Infof("Configure libp2p.ForceReachabilityPrivate()")
		opts = append(opts, libp2p.ForceReachabilityPrivate())
	}

	relayNums := len(relayMap)
	if c.MaxCandidates < relayNums {
		klog.Infof("MaxCandidates=%d is less than len(relayMap)=%d, set MaxCandidates to len(relayMap)",
			c.MaxCandidates, relayNums)
		c.MaxCandidates = relayNums
	}

	// configures libp2p to use the given private network protector
	if c.PSK.Enable {
		pskReader, err := GeneratePSKReader(c.PSK.Path)
		if err != nil {
			return nil, fmt.Errorf("failed to generate psk reader: %w", err)
		}
		psk, err := pnet.DecodeV1PSK(pskReader)
		if err != nil {
			return nil, fmt.Errorf("failed to decode v1 psk: %w", err)
		}
		opts = append(opts, libp2p.PrivateNetwork(psk))
	}

	var ddht *dual.DHT
	opts = append(opts, []libp2p.Option{
		libp2p.Identity(privKey),
		listenAddr,
		libp2p.DefaultSecurity,
		GenerateTransportOption(c.Transport), // edge tunnel 隧道协议，支持 tcp、websocket、QUIC
		libp2p.ConnectionManager(connMgr),
		libp2p.NATPortMap(),
		libp2p.Routing(func(h p2phost.Host) (routing.PeerRouting, error) {
			ddht, err = newDHT(ctx, h, relayMap)
			return ddht, err
		}),
		libp2p.EnableAutoRelay(
			autorelay.WithPeerSource(func(numPeers int) <-chan peer.AddrInfo {
				return peerSource
			}, 15*time.Second),
			autorelay.WithMinCandidates(0),
			autorelay.WithMaxCandidates(c.MaxCandidates),
			autorelay.WithBackoff(30*time.Second),
		),
		libp2p.EnableNATService(),   // 打洞相关
		libp2p.EnableHolePunching(), // 打洞相关
	}...)

	rcMgrOpts := make([]rcmgr.Option, 0)
	if c.MetricConfig.Enable {
		reporter, _ := obs.NewStatsTraceReporter()
		rcMgrOpts = append(rcMgrOpts, rcmgr.WithTraceReporter(reporter))
	}
	// Adjust stream limit
	if limitOpt, err := CreateLimitOpt(c.TunnelLimitConfig, rcMgrOpts...); err == nil {
		opts = append(opts, limitOpt)
	}
	h, err := libp2p.New(opts...) // 创建 libp2p 实例，每个实例都叫做一个 host
	if err != nil {
		return nil, fmt.Errorf("failed to new p2p host: %w", err)
	}
	klog.V(0).Infof("I'm %s\n", fmt.Sprintf("{%v: %v}", h.ID(), h.Addrs()))

	// If this host is a relay node, we need to run libp2p relayv2 service
	var relayService *relayv2.Relay
	if isRelay && c.Mode == defaults.ServerClientMode {
		relayService, err = relayv2.New(h, relayv2.WithLimit(nil)) // TODO close relayService
		if err != nil {
			return nil, fmt.Errorf("run libp2p relayv2 service error: %w", err)
		}
		klog.Infof("Run as a relay node")
	}

	// new hole punching service TODO fix hole punch not working
	ids, err := identify.NewIDService(h)
	if err != nil {
		return nil, fmt.Errorf("new id service error: %w", err)
	}
	holepunchService, err := holepunch.NewService(h, ids)
	if err != nil {
		return nil, fmt.Errorf("run libp2p holepunch service error: %w", err)
	}

	klog.Infof("Bootstrapping the DHT")
	if err = ddht.Bootstrap(ctx); err != nil {
		return nil, fmt.Errorf("failed to bootstrap dht: %w", err)
	}

	// connect to bootstrap 连接中继节点
	err = BootstrapConnect(ctx, h, relayMap)
	if err != nil {
		// We don't want to return error here, so that some
		// edge region that don't have access to the external
		// network can still work
		klog.Warningf("Failed to connect bootstrap: %v", err)
	}

	// init discovery services 初始化发现服务，通过相应的 run 方法发送服务发现请求
	mdnsPeerChan, err := initMDNS(h, c.Rendezvous) // 同个局域网使用 mDNS 进行对等点发现
	if err != nil {
		return nil, fmt.Errorf("init mdns discovery error: %w", err)
	}
	dhtPeerChan, err := initDHT(ctx, ddht, c.Rendezvous) // 跨局域网使用 DHT 进行对等点发现
	if err != nil {
		return nil, fmt.Errorf("init dht discovery error: %w", err)
	}

	// init config watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("init config watcher errror: %w", err)
	}
	err = watcher.Add(c.ConfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to add watch in %s, err: %w", c.ConfigPath, err)
	}

	edgeTunnel := &EdgeTunnel{
		Config:           c,
		p2pHost:          h,
		hostCtx:          ctx,
		nodePeerMap:      make(map[string]peer.ID),
		mdnsPeerChan:     mdnsPeerChan,
		dhtPeerChan:      dhtPeerChan,
		isRelay:          isRelay,
		relayMap:         relayMap,
		relayService:     relayService,
		holepunchService: holepunchService,
		stopCh:           make(chan struct{}),
		cfgWatcher:       watcher,
	}

	// run relay finder 运行过程中不断尝试连接中继节点，运行过程中可能有新的路由表信息，这样就可以连接到新的中继节点
	go edgeTunnel.runRelayFinder(ddht, peerSource, time.Duration(c.FinderPeriod)*time.Second)

	// register stream handlers
	if c.Mode == defaults.ServerClientMode {
		// 启动 pb 中的两个协议服务，进行对等点的发现，接收其他对等点发送过来的连接请求
		h.SetStreamHandler(defaults.DiscoveryProtocol, edgeTunnel.discoveryStreamHandler) // 对等点发现，记录对端发来的信息
		h.SetStreamHandler(defaults.ProxyProtocol, edgeTunnel.proxyStreamHandler)         // 对等点端到端连接处理
		h.SetStreamHandler(defaults.CNIProtocol, edgeTunnel.CNIAdapterStreamHandler)
	}
	Agent = edgeTunnel
	return edgeTunnel, nil
}

func generateListenAddr(c *v1alpha1.EdgeTunnelConfig) (libp2p.Option, error) {
	ips, err := GetIPsFromInterfaces(c.ListenInterfaces, c.ExtraFilteredInterfaces)
	if err != nil {
		return nil, fmt.Errorf("failed to get ips from listen interfaces: %w", err)
	}

	multiAddrStrings := make([]string, 0)
	if c.Mode == defaults.ServerClientMode {
		for _, ip := range ips {
			multiAddrStrings = append(multiAddrStrings, GenerateMultiAddrString(c.Transport, ip, c.ListenPort)) // port=20006
		}
	} else {
		for _, ip := range ips {
			multiAddrStrings = append(multiAddrStrings, GenerateMultiAddrString(c.Transport, ip, c.ListenPort+1))
		}
	}

	listenAddr := libp2p.ListenAddrStrings(multiAddrStrings...)
	return listenAddr, nil
}
