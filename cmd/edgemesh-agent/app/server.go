package app

import (
	"fmt"
	"github.com/kubeedge/edgemesh/pkg/monitor"
	"os"
	"strings"

	"github.com/spf13/cobra"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/cli/globalflag"
	"k8s.io/component-base/term"
	"k8s.io/klog/v2"

	"github.com/kubeedge/beehive/pkg/core"
	"github.com/kubeedge/edgemesh/cmd/edgemesh-agent/app/options"
	"github.com/kubeedge/edgemesh/pkg/apis/config/defaults"
	"github.com/kubeedge/edgemesh/pkg/apis/config/v1alpha1"
	"github.com/kubeedge/edgemesh/pkg/apis/config/v1alpha1/validation"
	"github.com/kubeedge/edgemesh/pkg/apis/module"
	"github.com/kubeedge/edgemesh/pkg/clients"
	"github.com/kubeedge/edgemesh/pkg/cni"
	"github.com/kubeedge/edgemesh/pkg/dns"
	"github.com/kubeedge/edgemesh/pkg/profile"
	"github.com/kubeedge/edgemesh/pkg/proxy"
	"github.com/kubeedge/edgemesh/pkg/tunnel"
	"github.com/kubeedge/edgemesh/pkg/util"
	netutil "github.com/kubeedge/edgemesh/pkg/util/net"
	kubeedgeutil "github.com/kubeedge/kubeedge/pkg/util"
	"github.com/kubeedge/kubeedge/pkg/util/flag"
	"github.com/kubeedge/kubeedge/pkg/version"
	"github.com/kubeedge/kubeedge/pkg/version/verflag"
)

func NewEdgeMeshAgentCommand() *cobra.Command {
	opts := options.NewEdgeMeshAgentOptions()
	cmd := &cobra.Command{
		Use:  "edgemesh-agent",
		Long: `edgemesh-agent is the data plane component of EdgeMesh.`,
		Run: func(cmd *cobra.Command, args []string) {
			verflag.PrintAndExitIfRequested()
			flag.PrintMinConfigAndExitIfRequested(v1alpha1.NewDefaultEdgeMeshAgentMinConfig())
			flag.PrintDefaultConfigAndExitIfRequested(v1alpha1.NewDefaultEdgeMeshAgentConfig(""))
			flag.PrintFlags(cmd.Flags())

			if errs := opts.Validate(); len(errs) > 0 {
				klog.Exit(kubeedgeutil.SpliceErrors(errs))
			}

			// 通过用户提供的配置文件 /etc/edgemesh/config/edgemesh-agent.yaml 创建 EdgeMeshAgentConfig 对象
			cfg, err := opts.Config()
			if err != nil {
				klog.Exit(err)
			}

			if errs := validation.ValidateEdgeMeshAgentConfiguration(cfg); len(errs) > 0 {
				klog.Exit(kubeedgeutil.SpliceErrors(errs.ToAggregate().Errors()))
			}

			klog.Infof("Version: %+v", version.Get())
			if err = Run(cfg); err != nil {
				klog.Exit("run edgemesh-agent failed: ", err)
			}
		},
	}
	fs := cmd.Flags()
	namedFs := opts.Flags()
	verflag.AddFlags(namedFs.FlagSet("global"))
	flag.AddFlags(namedFs.FlagSet("global"))
	globalflag.AddGlobalFlags(namedFs.FlagSet("global"), cmd.Name())
	for _, f := range namedFs.FlagSets {
		fs.AddFlagSet(f)
	}

	usageFmt := "Usage:\n  %s\n"
	cols, _, _ := term.TerminalSize(cmd.OutOrStdout())
	cmd.SetUsageFunc(func(cmd *cobra.Command) error {
		fmt.Fprintf(cmd.OutOrStderr(), usageFmt, cmd.UseLine())
		cliflag.PrintSections(cmd.OutOrStderr(), namedFs, cols)
		return nil
	})
	cmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		fmt.Fprintf(cmd.OutOrStdout(), "%s\n\n"+usageFmt, cmd.Long, cmd.UseLine())
		cliflag.PrintSections(cmd.OutOrStdout(), namedFs, cols)
	})

	return cmd
}

// Run runs edgemesh-agent
func Run(cfg *v1alpha1.EdgeMeshAgentConfig) error {
	defer klog.Infof("edgemesh-agent exited")

	fmt.Println("Hello WeixinX!")

	trace := 1

	klog.Infof("[%d] Prepare agent to run", trace)
	// 确定 Kube-API 的模式（手动、云、边），若是边缘模式需要生成 kubeconfig
	// 创建 edgemesh0 网桥，监听 53 DNS 端口和
	// Proxier 启用的随机端口（详见 proxy/proxysocket.go/newProxySocket）
	// 设置 Tunnel 的模式
	if err := prepareRun(cfg); err != nil {
		return err
	}
	klog.Infof("edgemesh-agent running on %s", cfg.KubeAPIConfig.Mode)
	trace++

	klog.Infof("[%d] New clients", trace)
	// 创建 Kube 和 Istio Client
	cli, err := clients.NewClients(cfg.KubeAPIConfig)
	if err != nil {
		return err
	}
	trace++

	klog.Infof("[%d] Register beehive modules", trace)
	nodeNmae, ok := os.LookupEnv("NODE_NAME")
	if !ok {
		klog.Fatal("env NODE_NAME not exist")
	}
	clusterName, ok := os.LookupEnv("CLUSTER_NAME")
	if !ok {
		klog.Fatal("env CLUSTER_NAME not exist")
	}
	store := monitor.NewMetricsStore(nodeNmae, clusterName)
	go store.Run()
	defer store.Stop()
	localMonitor := monitor.NewLocalMonitor("http://127.0.0.1:8080", store)
	go localMonitor.Run()
	defer localMonitor.Stop()
	// 注册 EdgeDNS、EdgeProxy、EdgeTunnel、EdgeCNI 模块，重点关注 proxy 和 tunnel
	if errs := registerModules(cfg, cli, store); len(errs) > 0 {
		return fmt.Errorf(kubeedgeutil.SpliceErrors(errs))
	}
	trace++

	klog.Infof("[%d] Cache beehive modules", trace)
	// 将模块存入 map 中，name -> module
	if err = module.Initialize(core.GetModules()); err != nil {
		return err
	}
	defer module.Shutdown()
	trace++

	klog.Infof("[%d] Start all modules", trace)
	// 为每个模块用一个协程调用 Start 方法
	// 同时设置优雅退出
	core.Run()

	return nil
}

// registerModules register all the modules started in edgemesh-agent
func registerModules(c *v1alpha1.EdgeMeshAgentConfig, cli *clients.Clients, store *monitor.MetricsStore) []error {

	var errs []error
	if err := dns.Register(c.Modules.EdgeDNSConfig, cli); err != nil {
		errs = append(errs, err)
	}
	if err := proxy.Register(c.Modules.EdgeProxyConfig, cli, store); err != nil {
		errs = append(errs, err)
	}
	if err := tunnel.Register(c.Modules.EdgeTunnelConfig, store); err != nil {
		errs = append(errs, err)
	}
	if err := cni.Register(c.Modules.EdgeCNIConfig, cli); err != nil {
		errs = append(errs, err)
	}
	return errs
}

// prepareRun prepares edgemesh-agent to run
func prepareRun(c *v1alpha1.EdgeMeshAgentConfig) error {
	// start pprof server
	profile.ListenAndServer(c.CommonConfig.PprofConfig)

	// 确定 Kube-API 的模式（手动、云、边），若是边缘模式需要生成 kubeconfig
	// Enter manual mode if user set Master or KubeConfig
	if c.KubeAPIConfig.Master != "" || c.KubeAPIConfig.KubeConfig != "" {
		c.KubeAPIConfig.Mode = defaults.ManualMode
	} else {
		if c.KubeAPIConfig.Mode == defaults.EdgeMode {
			// If the security feature of metaServer is set, then the address
			// of metaServer must be replaced with the https schema
			if c.KubeAPIConfig.MetaServer.Security.RequireAuthorization {
				c.KubeAPIConfig.MetaServer.Server = strings.ReplaceAll(c.KubeAPIConfig.MetaServer.Server, "http://", "https://")
			}
			// Create a kubeConfig file on local path for subsequent builds of K8s
			// client-go's kubeClient. If it already exists, we don't create it again.
			if _, err := os.Stat(defaults.TempKubeConfigPath); err != nil && os.IsNotExist(err) {
				err = util.SaveKubeConfigFile(util.GenerateKubeClientConfig(c.KubeAPIConfig))
				if err != nil {
					return fmt.Errorf("failed to create kubeConfig: %w", err)
				}
			}
			c.KubeAPIConfig.KubeConfig = defaults.TempKubeConfigPath
		}
	}
	// 否则运行在 CloudMode 模式下

	// Set dns and proxy modules listenInterface
	// 创建 edgemesh0 网桥，监听 53 DNS 端口和
	// Proxier 启用的随机端口（详见 proxy/proxysocket.go/newProxySocket）
	// 设置 Tunnel 的模式
	err := netutil.CreateEdgeMeshDevice(c.CommonConfig.BridgeDeviceName, c.CommonConfig.BridgeDeviceIP)
	if err != nil {
		return fmt.Errorf("failed to create edgemesh device %s: %w", c.CommonConfig.BridgeDeviceName, err)
	}
	c.Modules.EdgeDNSConfig.ListenInterface = c.CommonConfig.BridgeDeviceName
	c.Modules.EdgeProxyConfig.ListenInterface = c.CommonConfig.BridgeDeviceName

	// Set dns module KubeAPIConfig
	c.Modules.EdgeDNSConfig.KubeAPIConfig = c.KubeAPIConfig

	// Set node name and namespace
	nodeName, exists := os.LookupEnv("NODE_NAME")
	if !exists {
		return fmt.Errorf("env NODE_NAME not exist")
	}
	namespace, exists := os.LookupEnv("NAMESPACE")
	if !exists {
		return fmt.Errorf("env NAMESPACE not exist")
	}
	c.Modules.EdgeProxyConfig.LoadBalancer.NodeName = nodeName
	c.Modules.EdgeProxyConfig.Socks5Proxy.NodeName = nodeName
	c.Modules.EdgeProxyConfig.Socks5Proxy.Namespace = namespace
	c.Modules.EdgeTunnelConfig.NodeName = nodeName

	// Set tunnel module mode
	c.Modules.EdgeTunnelConfig.Mode = defaults.ServerClientMode

	return nil
}
