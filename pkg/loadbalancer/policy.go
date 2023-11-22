package loadbalancer

import (
	"bufio"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"strings"
	"sync"

	"github.com/buraksezer/consistent"
	istioapi "istio.io/client-go/pkg/apis/networking/v1alpha3"
	"k8s.io/klog/v2"

	"github.com/kubeedge/edgemesh/pkg/apis/config/v1alpha1"
)

const (
	RoundRobin     = "ROUND_ROBIN"
	Random         = "RANDOM"
	ConsistentHash = "CONSISTENT_HASH" // 基于一致性哈希环算法的会话保持
	MultiLevel     = "MULTI_LEVEL"     // 多层次负载均衡

	HttpHeader   = "HTTP_HEADER"
	UserSourceIP = "USER_SOURCE_IP"
)

type Policy interface {
	Name() string
	Update(oldDr, dr *istioapi.DestinationRule)
	Pick(endpoints []string, srcAddr net.Addr, tcpConn net.Conn, cliReq *http.Request) (string, *http.Request, error)
	Sync(endpoints []string)
	Release()
}

// RoundRobinPolicy is a default policy.
type RoundRobinPolicy struct {
}

func NewRoundRobinPolicy() *RoundRobinPolicy {
	return &RoundRobinPolicy{}
}

func (*RoundRobinPolicy) Name() string {
	return RoundRobin
}

func (*RoundRobinPolicy) Update(oldDr, dr *istioapi.DestinationRule) {}

func (*RoundRobinPolicy) Pick(endpoints []string, srcAddr net.Addr, netConn net.Conn, cliReq *http.Request) (string, *http.Request, error) {
	// RoundRobinPolicy is an empty implementation and we won't use it,
	// the outer round-robin policy will be used next.
	return "", cliReq, fmt.Errorf("call RoundRobinPolicy is forbidden")
}

func (*RoundRobinPolicy) Sync(endpoints []string) {}

func (*RoundRobinPolicy) Release() {}

type RandomPolicy struct {
	lock sync.Mutex
}

func NewRandomPolicy() *RandomPolicy {
	return &RandomPolicy{}
}

func (rd *RandomPolicy) Name() string {
	return Random
}

func (rd *RandomPolicy) Update(oldDr, dr *istioapi.DestinationRule) {}

func (rd *RandomPolicy) Pick(endpoints []string, srcAddr net.Addr, netConn net.Conn, cliReq *http.Request) (string, *http.Request, error) {
	rd.lock.Lock()
	k := rand.Int() % len(endpoints)
	rd.lock.Unlock()
	return endpoints[k], cliReq, nil
}

func (rd *RandomPolicy) Sync(endpoints []string) {}

func (rd *RandomPolicy) Release() {}

type ConsistentHashPolicy struct {
	Config   *v1alpha1.ConsistentHash
	lock     sync.Mutex
	hashRing *consistent.Consistent
	hashKey  HashKey
}

func NewConsistentHashPolicy(config *v1alpha1.ConsistentHash, dr *istioapi.DestinationRule, endpoints []string) *ConsistentHashPolicy {
	return &ConsistentHashPolicy{
		Config:   config,
		hashRing: newHashRing(config, endpoints),
		hashKey:  getConsistentHashKey(dr),
	}
}

func (ch *ConsistentHashPolicy) Name() string {
	return ConsistentHash
}

func (ch *ConsistentHashPolicy) Update(oldDr, dr *istioapi.DestinationRule) {
	ch.lock.Lock()
	ch.hashKey = getConsistentHashKey(dr)
	ch.lock.Unlock()
}

func (ch *ConsistentHashPolicy) Pick(endpoints []string, srcAddr net.Addr, netConn net.Conn, cliReq *http.Request) (endpoint string, req *http.Request, err error) {
	ch.lock.Lock()
	defer ch.lock.Unlock()

	req = cliReq
	var keyValue string
	switch ch.hashKey.Type {
	case HttpHeader:
		if req == nil {
			req, err = http.ReadRequest(bufio.NewReader(netConn))
			if err != nil {
				klog.Errorf("read http request err: %v", err)
				return "", nil, err
			}
		}
		keyValue = req.Header.Get(ch.hashKey.Key)
	case UserSourceIP:
		if srcAddr == nil && netConn != nil {
			srcAddr = netConn.RemoteAddr()
		}
		keyValue = srcAddr.String()
	default:
		klog.Errorf("Failed to get hash key value")
		keyValue = ""
	}
	klog.Infof("Get key value: %s", keyValue)
	member := ch.hashRing.LocateKey([]byte(keyValue))
	if member == nil {
		errMsg := fmt.Errorf("can't find a endpoint by given key: %s", keyValue)
		klog.Errorf("%v", errMsg)
		return "", req, errMsg
	}
	return member.String(), req, nil
}

func (ch *ConsistentHashPolicy) Sync(endpoints []string) {
	ch.lock.Lock()
	if ch.hashRing == nil {
		ch.hashRing = newHashRing(ch.Config, endpoints)
	} else {
		updateHashRing(ch.hashRing, endpoints)
	}
	ch.lock.Unlock()
}

func (ch *ConsistentHashPolicy) Release() {
	ch.lock.Lock()
	clearHashRing(ch.hashRing)
	ch.lock.Unlock()
}

const (
	NodeCpuThreshold float64 = 60
	NodeMemThreshold float64 = 60
	PodCpuThreshold  float64 = 60
	PodMemThreshold  float64 = 60
)

// TODO(WeixinX)
// 多层次负载均衡策略
type MultiLevelPolicy struct {
	// TODO(WeixinX): 需要将部分成员变量放到 LB 结构中，例如 metrics、inCluster、outCluster，这样可以方便更新，
	//  同时可以在创建新的 svc 时，直接引用这些变量来创建 MultiLevelPolicy
	selfName    string
	clusterName string
	metrics     *Metrics
	lock        sync.RWMutex // protects inCluster
}

type Metrics struct {
	node       map[string]*NodeInfo // nodeName -> NodeInfo
	inCluster  *InClusterInfo
	outCluster *OutClusterInfo
}

type NodeInfo struct {
	nodeName    string
	clusterName string
	cpuUsage    float64
	memUsage    float64
	appInfo     map[string]*AppInfo // appName -> AppInfo
}

type AppInfo struct {
	appName     string
	nodeName    string
	cpuAvgUsage float64
	memAvgUsage float64
	podInfo     []PodInfo
	index       int
}

type PodInfo struct {
	podName  string
	nodeName string
	endpoint string // nodeName:podName:ip:port
	cpuUsage float64
	memUsage float64
}

type InClusterInfo struct {
	nodeCpuAvgUsage float64
	nodeMemAvgUsage float64
	appInfo         map[string]*AppInfo // appName -> AppInfo
}

type OutClusterInfo struct {
	bestNode map[string]*NodeInfo // appName -> NodeInfo，后台根据节点/应用资源情况计算出来的最佳节点
}

func NewMultiLevelPolicy() *MultiLevelPolicy {
	fmt.Println("[Info] [NewMultiLevelPolicy] new a mock struct for test")
	return mockMultiLevelPolicy
}

func (m *MultiLevelPolicy) Name() string {
	return MultiLevel
}

func (m *MultiLevelPolicy) Pick(endpoints []string, srcAddr net.Addr, tcpConn net.Conn, cliReq *http.Request) (string, *http.Request, error) {
	if len(endpoints) == 0 {
		fmt.Println("[Error] [MultiLevelPolicy.Pick] endpoints is empty")
		return "", cliReq, fmt.Errorf("endpoints is empty")
	}
	tmp := strings.Split(endpoints[0], ":")
	if len(tmp) != 4 {
		fmt.Println("[Error] [MultiLevelPolicy.Pick] endpoint format is invalid")
		return "", cliReq, fmt.Errorf("endpoint format is invalid")
	}
	podName := tmp[1]
	tmp = strings.Split(podName, "-")
	if len(tmp) < 2 {
		fmt.Println("[Error] [MultiLevelPolicy.Pick] pod name format is invalid")
		return "", cliReq, fmt.Errorf("pod name format is invalid")
	}
	appName := strings.Join(tmp[:len(tmp)-2], "-") // 去掉最后的 pod-id 和 replicaset-id
	fmt.Printf("[Info] [MultiLevelPolicy.Pick] get app name: %s\n", appName)

	// 1. 若本地存在对应的 App Pod，且当本地节点负载 < 阈值时，且本地 Pods 的平均负载 < 阈值时，在本地 Pods 间进行轮询
	localMetrics := m.metrics.node[m.selfName]
	if localMetrics == nil {
		fmt.Printf("[Error] [MultiLevelPolicy.Pick] the metrics for local %s can't be found\n", m.selfName)
		return "", cliReq, fmt.Errorf("the metrics for local can't be found")
	}
	if localMetrics.appInfo[appName] != nil &&
		m.localNodeResourceLTThreshold() && m.localPodResourceLTThreshold(appName) {
		return m.pickLocal(appName), cliReq, nil
	}

	// 2. 若(本地不存在对应的 App Pod，或本地节点负载 >= 阈值时，或本地 Pods 的平均负载 >= 阈值)，且(簇内节点负载 < 阈值，且簇内 Pods 的平均负载 < 阈值)时，在簇内节点（Pods）间进行轮询
	// localMetrics.appInfo[appName] == nil || !m.localNodeResourceLTThreshold() || !m.localPodResourceLTThreshold(appName)
	if m.metrics.inCluster.appInfo[appName] != nil &&
		m.inClusterNodeResourceLTThreshold() && m.inClusterPodResourceLTThreshold(appName) {
		return m.pickInCluster(appName), cliReq, nil
	}

	// 3. 若簇内节点（Pods）负载 >= 阈值时，或簇内 Pods 的平均负载 >= 阈值时，
	// 选择一个最佳（考虑节点 CPU、内存，带宽、心跳往返时延，Pod CPU、内存）簇外节点进行转发
	// m.inCluster.appInfo[appName] != nil || m.inClusterNodeResourceLTThreshold() || m.inClusterPodResourceLTThreshold(appName)
	return m.pickOutCluster(appName), cliReq, nil
}

func (m *MultiLevelPolicy) nodeResourceLTThreshold(nodeName string) bool {
	metrics := m.metrics.node[m.selfName]
	if metrics == nil {
		fmt.Printf("[Error] [MultiLevelPolicy.nodeResourceLTThreshold] the metrics for node %s can't be found\n", m.selfName)
		return false
	}
	return metrics.cpuUsage < NodeCpuThreshold && metrics.memUsage < NodeMemThreshold
}

func (m *MultiLevelPolicy) podResourceLTThreshold(cpu, mem float64) bool {
	return cpu < PodCpuThreshold && mem < PodMemThreshold
}

func (m *MultiLevelPolicy) localNodeResourceLTThreshold() bool {
	return m.nodeResourceLTThreshold(m.selfName)
}

func (m *MultiLevelPolicy) localPodResourceLTThreshold(appName string) bool {
	return m.metrics.node[m.selfName].appInfo[appName].cpuAvgUsage < PodCpuThreshold &&
		m.metrics.node[m.selfName].appInfo[appName].memAvgUsage < PodMemThreshold
}

func (m *MultiLevelPolicy) pickLocal(appName string) string {
	fmt.Printf("[Info] [MultiLevelPolicy.pickLocal] pick app %s pod in local\n", appName)
	var podInfo PodInfo
	appInfo := m.metrics.node[m.selfName].appInfo[appName]
	for {
		podInfo = appInfo.podInfo[appInfo.index]
		appInfo.index = (appInfo.index + 1) % len(appInfo.podInfo)
		if m.podResourceLTThreshold(podInfo.cpuUsage, podInfo.memUsage) {
			break
		}
	}
	fmt.Printf("[Info] [MultiLevelPolicy.pickLocal] pick endpoint: %s\n", podInfo.endpoint)
	return podInfo.endpoint
}

func (m *MultiLevelPolicy) inClusterNodeResourceLTThreshold() bool {
	return m.metrics.inCluster.nodeCpuAvgUsage < NodeCpuThreshold &&
		m.metrics.inCluster.nodeMemAvgUsage < NodeMemThreshold
}

func (m *MultiLevelPolicy) inClusterPodResourceLTThreshold(appName string) bool {
	return m.metrics.inCluster.appInfo[appName].cpuAvgUsage < PodCpuThreshold &&
		m.metrics.inCluster.appInfo[appName].memAvgUsage < PodMemThreshold
}

func (m *MultiLevelPolicy) pickInCluster(appName string) string {
	fmt.Printf("[Info] [MultiLevelPolicy.pickInCluster] pick app %s pod in cluster\n", appName)
	var podInfo PodInfo
	appInfo := m.metrics.inCluster.appInfo[appName]
	for {
		if appInfo.nodeName == m.selfName {
			continue
		}
		if !m.nodeResourceLTThreshold(appInfo.nodeName) {
			continue
		}

		podInfo = appInfo.podInfo[appInfo.index]
		appInfo.index = (appInfo.index + 1) % len(appInfo.podInfo)
		if m.podResourceLTThreshold(podInfo.cpuUsage, podInfo.memUsage) {
			break
		}
	}
	fmt.Printf("[Info] [MultiLevelPolicy.pickInCluster] pick endpoint: %s\n", podInfo.endpoint)
	return podInfo.endpoint
}

func (m *MultiLevelPolicy) pickOutCluster(appName string) string {
	fmt.Printf("[Info] [MultiLevelPolicy.pickOutCluster] pick app %s pod in out cluster\n", appName)
	var podInfo PodInfo
	appInfo := m.metrics.outCluster.bestNode[appName].appInfo[appName]
	for {
		podInfo = appInfo.podInfo[appInfo.index]
		appInfo.index = (appInfo.index + 1) % len(appInfo.podInfo)
		if m.podResourceLTThreshold(podInfo.cpuUsage, podInfo.memUsage) {
			break
		}
	}
	fmt.Printf("[Info] [MultiLevelPolicy.pickOutCluster] pick endpoint: %s\n", podInfo.endpoint)
	return podInfo.endpoint
}

func (m *MultiLevelPolicy) Update(oldDr, dr *istioapi.DestinationRule) {
	// TODO implement me
}

func (m *MultiLevelPolicy) Sync(endpoints []string) {
	// TODO implement me
}

func (m *MultiLevelPolicy) Release() {
	// TODO implement me
}
