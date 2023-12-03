package tunnel

import (
	"fmt"
	"github.com/jinzhu/copier"
	"time"

	"github.com/kubeedge/edgemesh/pkg/apis/config/defaults"
	"github.com/kubeedge/edgemesh/pkg/monitor"
	"github.com/kubeedge/edgemesh/pkg/tunnel/pb/heartbeat"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/libp2p/go-msgio/pbio"
	ma "github.com/multiformats/go-multiaddr"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
)

const (
	heartbeatInterval = 3 * time.Second
)

func (t *EdgeTunnel) runMyHeartbeat() {
	logHeader := "[EdgeTunnel.runMyHeartbeat]"

	condition := func() (done bool, err error) {
		for _, peerId := range t.nodePeerMap {
			if peerId == t.p2pHost.ID() {
				continue
			}

			peerInfo := t.p2pHost.Peerstore().PeerInfo(peerId)
			klog.Infof("%s start heartbeat to %s, addrs: %v", logHeader, peerInfo.ID, peerInfo.Addrs)
			if err = AddCircuitAddrsToPeer(&peerInfo, t.relayMap); err != nil {
				klog.Errorf("%s failed to add circuit addrs to peer %s", logHeader, peerInfo)
				continue
			}
			t.p2pHost.Peerstore().AddAddrs(peerInfo.ID, peerInfo.Addrs, peerstore.PermanentAddrTTL)
			go t.heartbeat(peerInfo)
		}
		return false, nil
	}
	err := wait.PollUntil(heartbeatInterval, condition, t.stopCh)
	if err != nil {
		klog.Errorf("%s causes an error %v", logHeader, err)
	}
}

func (t *EdgeTunnel) heartbeat(pi peer.AddrInfo) {
	logHeader := "[EdgeTunnel.heartbeat]"

	if err := t.p2pHost.Connect(t.hostCtx, pi); err != nil {
		klog.Errorf("%s failed to connect to peer %s: %v", logHeader, pi.ID, err)
		return
	}

	stream, err := t.p2pHost.NewStream(network.WithUseTransient(t.hostCtx, "relay"), pi.ID, defaults.HeartbeatProtocol)
	if err != nil {
		klog.Errorf("%s failed to create stream to peer %s: %v", logHeader, pi.ID, err)
		return
	}
	defer func() {
		if err = stream.Close(); err != nil {
			klog.Errorf("%s failed to close stream between %s: %v", logHeader, pi.ID, err)
		}
		// if err = stream.Reset(); err != nil {
		// 	klog.Errorf("%s failed to reset stream between %s: %v", logHeader, pi.ID, err)
		// }
	}()
	klog.Infof("%s New stream between peer %s success", logHeader, pi.ID)

	streamWriter := pbio.NewDelimitedWriter(stream)
	streamReader := pbio.NewDelimitedReader(stream, MaxReadSize)

	// send heartbeat
	msg := new(heartbeat.Heartbeat)
	t.store.Lock.RLock()
	if t.store.Local == nil {
		t.store.Lock.RUnlock()
		return
	}
	if err = copier.CopyWithOption(msg, t.store.Local, copier.Option{DeepCopy: true}); err != nil {
		klog.Errorf("%s failed to copy local heartbeat msg: %v", logHeader, err)
	}
	t.store.Lock.RUnlock()
	if err = streamWriter.WriteMsg(msg); err != nil {
		// resetErr := stream.Reset()
		// if resetErr != nil {
		// 	klog.Errorf("%s stream between %s reset err: %w", logHeader, msg.Node.Name, resetErr)
		// }
		klog.Errorf("%s failed to write msg to %s: %v", logHeader, pi.ID, err)
		return
	}
	klog.Infof("%s send a heartbeat msg to %s: %s", logHeader, pi.ID, msg.String())

	// read heartbeat
	msg.Reset()
	if err = streamReader.ReadMsg(msg); err != nil {
		// resetErr := stream.Reset()
		// if resetErr != nil {
		// 	klog.Errorf("%s stream between %s reset err: %w", logHeader, pi, resetErr)
		// }
		klog.Errorf("%s failed to read response msg from %s: %v", logHeader, pi.ID, err)
		return
	}
	msgStatus := msg.GetStatus()
	if msgStatus != heartbeat.Status_OK {
		// resetErr := stream.Reset()
		// if resetErr != nil {
		// 	klog.Errorf("%s stream between %s reset err: %w", logHeader, pi, resetErr)
		// }
		klog.Errorf("%s receive a heartbeat msg from %s, but status is %s", logHeader, msg.GetNode().GetName(), msgStatus.String())
		return
	}
	klog.Infof("%s receive a heartbeat msg from %s: %s", logHeader, msg.GetNode().GetName(), msg.String())
	if err = t.updateHeartbeatMsgToStore(msg); err != nil {
		// resetErr := stream.Reset()
		// if resetErr != nil {
		// 	klog.Errorf("%s stream between %s reset err: %w", logHeader, pi, resetErr)
		// }
		klog.Errorf("%s failed to update heartbeat msg to monitor store: %v", logHeader, err)
		return
	}
	klog.Infof("%s update node %s heartbeat to monitor store", logHeader, msg.GetNode().GetName())
}

func (t *EdgeTunnel) heartbeatStreamHandler(stream network.Stream) {
	logHeader := "[heart handler]"
	remotePeer := peer.AddrInfo{
		ID:    stream.Conn().RemotePeer(),
		Addrs: []ma.Multiaddr{stream.Conn().RemoteMultiaddr()},
	}
	klog.Infof("%s heartbeat service got a new stream from %s", logHeader, remotePeer)
	defer func() {
		if err := stream.Close(); err != nil {
			klog.Errorf("%s failed to close stream between %s: %v", logHeader, remotePeer.ID, err)
		}
		// if err = stream.Reset(); err != nil {
		// 	klog.Errorf("%s failed to reset stream between %s: %v", logHeader, pi.ID, err)
		// }
	}()

	streamWriter := pbio.NewDelimitedWriter(stream)
	streamReader := pbio.NewDelimitedReader(stream, MaxReadSize)

	// read heartbeat
	msg := new(heartbeat.Heartbeat)
	if err := streamReader.ReadMsg(msg); err != nil {
		klog.Errorf("%s failed to read msg from %s: %v", logHeader, remotePeer.ID, err)
		return
	}
	msgStatus := msg.GetStatus()
	if msgStatus != heartbeat.Status_OK {
		klog.Errorf("%s receive a heartbeat msg from %s, but status is %s", logHeader, msg.GetNode().GetName(), msgStatus.String())
		return
	}
	klog.Infof("%s receive a heartbeat msg from %s: %s", logHeader, msg.GetNode().GetName(), msg.String())
	if err := t.updateHeartbeatMsgToStore(msg); err != nil {
		klog.Errorf("%s failed to update heartbeat msg to monitor store: %v", logHeader, err)
		return
	}
	klog.Infof("%s update node %s heartbeat to monitor store", logHeader, msg.GetNode().GetName())

	// write heartbeat
	msg.Reset()
	t.store.Lock.RLock()
	if t.store.Local == nil {
		t.store.Lock.RUnlock()
		return
	}
	if err := copier.CopyWithOption(msg, t.store.Local, copier.Option{DeepCopy: true}); err != nil {
		klog.Errorf("%s failed to copy local heartbeat msg: %v", logHeader, err)
	}
	t.store.Lock.RUnlock()
	if err := streamWriter.WriteMsg(msg); err != nil {
		klog.Errorf("%s failed to write msg to %s: %v", logHeader, remotePeer.ID, err)
		return
	}
	klog.Infof("%s send a heartbeat msg to %s: %s", logHeader, remotePeer.ID, msg.String())
}

func (t *EdgeTunnel) updateHeartbeatMsgToStore(msg *heartbeat.Heartbeat) error {
	if msg == nil {
		return fmt.Errorf("heartbeat msg is nil")
	}

	nodeName := msg.GetNode().GetName()
	if nodeName == "" {
		return fmt.Errorf("node name is empty")
	}
	clusterName := msg.GetNode().GetClusterName()
	if clusterName == "" {
		return fmt.Errorf("cluster name is empty")
	}

	t.store.Lock.Lock()
	defer t.store.Lock.Unlock()
	if _, exist := t.store.Metrics[nodeName]; !exist {
		t.store.Metrics[nodeName] = &monitor.MetricsItem{
			Node: &monitor.NodeInfo{
				Name:        nodeName,
				ClusterName: clusterName,
				Apps:        make(map[string]*monitor.AppInfo),
			},
		}
	}
	state := t.store.Metrics[nodeName]
	state.Status = "OK"
	state.Node.CpuPercent = msg.GetNode().GetCpuPercent()
	state.Node.MemPercent = msg.GetNode().GetMemPercent()

	// TODO(WeixinX): 应该更新所有 app 信息，这就意味着得反复创建 AppInfo 结构
	recApps := msg.GetApps()
	for _, rApp := range recApps {
		appName := rApp.GetName()
		if _, exist := state.Node.Apps[appName]; !exist {
			state.Node.Apps[appName] = &monitor.AppInfo{
				Name:     appName,
				NodeName: nodeName,
				PodIdx:   make(map[string]int),
				Pods:     make([]monitor.PodInfo, 0),
			}
		}
		app := state.Node.Apps[appName]
		for _, rPod := range rApp.GetPods() {
			podName := rPod.GetName()
			if _, exist := app.PodIdx[rPod.Name]; !exist {
				app.Pods = append(app.Pods, monitor.PodInfo{
					Name:     podName,
					NodeName: nodeName,
					Endpoint: fmt.Sprintf("%s:%s:%s:%s", nodeName, podName, rPod.GetIp(), rPod.GetPort()),
				})
				app.PodIdx[podName] = len(app.Pods) - 1
			}
			app.Pods[app.PodIdx[podName]].CpuPercent = rPod.GetCpuPercent()
			app.Pods[app.PodIdx[podName]].MemPercent = rPod.GetMemPercent()
		}
	}

	for _, app := range state.Node.Apps {
		ttCpuPercent := 0.0
		ttMemPercent := 0.0
		for _, pod := range app.Pods {
			ttCpuPercent += pod.CpuPercent
			ttMemPercent += pod.MemPercent
		}
		if len(app.Pods) == 0 {
			continue
		}
		app.CpuAvgPercent = ttCpuPercent / float64(len(app.Pods))
		app.MemAvgPercent = ttMemPercent / float64(len(app.Pods))
	}

	return nil
}
