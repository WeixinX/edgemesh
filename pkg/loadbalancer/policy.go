package loadbalancer

import (
	"bufio"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"strings"
	"sync"

	"github.com/kubeedge/edgemesh/pkg/apis/config/v1alpha1"
	"github.com/kubeedge/edgemesh/pkg/monitor"

	"github.com/buraksezer/consistent"
	istioapi "istio.io/client-go/pkg/apis/networking/v1alpha3"
	"k8s.io/klog/v2"
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
	PodCpuThreshold  float64 = 0.6
	PodMemThreshold  float64 = 0.6
)

// TODO(WeixinX)
// 多层次负载均衡策略
type MultiLevelPolicy struct {
	selfName    string
	clusterName string
	store       *monitor.MetricsStore
}

func NewMultiLevelPolicy(store *monitor.MetricsStore) *MultiLevelPolicy {
	logHeader := "[NewMultiLevelPolicy]"
	// fmt.Println("[Info] [NewMultiLevelPolicy] new a mock struct for test")
	// return mockMultiLevelPolicy
	klog.Infof("%s new a real struct", logHeader)
	return &MultiLevelPolicy{
		selfName:    store.SelfName,
		clusterName: store.ClusterName,
		store:       store,
	}
}

func (m *MultiLevelPolicy) Name() string {
	return MultiLevel
}

func (m *MultiLevelPolicy) Pick(endpoints []string, srcAddr net.Addr, tcpConn net.Conn, cliReq *http.Request) (string, *http.Request, error) {
	logHeader := "[MultiLevelPolicy.Pick]"
	if len(endpoints) == 0 {
		klog.Errorf("%s endpoints is empty", logHeader)
		return "", cliReq, fmt.Errorf("endpoints is empty")
	}
	tmp := strings.Split(endpoints[0], ":")
	if len(tmp) != 4 {
		klog.Errorf("%s endpoint(%s) format is invalid", logHeader, endpoints[0])
		return "", cliReq, fmt.Errorf("endpoint format is invalid")
	}
	podName := tmp[1]
	tmp = strings.Split(podName, "-")
	if len(tmp) < 2 {
		klog.Errorf("%s pod name(%s) format is invalid", logHeader, podName)
		return "", cliReq, fmt.Errorf("pod name format is invalid")
	}
	appName := strings.Join(tmp[:len(tmp)-2], "-") // 去掉最后的 pod-id 和 replicaset-id
	klog.Infof("%s get app name: %s", logHeader, appName)

	m.store.Lock.RLock()
	defer m.store.Lock.RUnlock()
	// 1. 若本地存在对应的 App Pod，且当本地节点负载 < 阈值时，且本地 Pods 的平均负载 < 阈值时，在本地 Pods 间进行轮询
	if m.localExistAppPods(appName) && m.localNodeResourceLTThreshold() && m.localPodResourceLTThreshold(appName) {
		return m.pickLocal(appName), cliReq, nil
	}

	// 2. 若(本地不存在对应的 App Pod，或本地节点负载 >= 阈值时，或本地 Pods 的平均负载 >= 阈值)，且(簇内节点负载 < 阈值，且簇内 Pods 的平均负载 < 阈值)时，在簇内节点（Pods）间进行轮询
	// !m.localExistAppPods(appName) || !m.localNodeResourceLTThreshold() || !m.localPodResourceLTThreshold(appName)
	if m.inClusterExistAppPods(appName) && m.inClusterNodeResourceLTThreshold() && m.inClusterPodResourceLTThreshold(appName) {
		return m.pickInCluster(appName), cliReq, nil
	}

	// 3. 若簇内节点（Pods）负载 >= 阈值时，或簇内 Pods 的平均负载 >= 阈值时，
	// 选择一个最佳（考虑节点 CPU、内存，带宽、心跳往返时延，Pod CPU、内存）簇外节点进行转发
	// !m.inClusterExistAppPods(appName) || !m.inClusterNodeResourceLTThreshold() || !m.inClusterPodResourceLTThreshold(appName)
	return m.pickOutCluster(appName), cliReq, nil
}

func (m *MultiLevelPolicy) nodeResourceLTThreshold(nodeName string) bool {
	logHeader := "[MultiLevelPolicy.nodeResourceLTThreshold]"
	metrics := m.store.Metrics[nodeName]
	if metrics == nil || metrics.Status != "OK" || metrics.Node == nil {
		klog.Errorf("%s the metrics for node %s can't be found", logHeader, nodeName)
		return false
	}
	return metrics.Node.CpuPercent < NodeCpuThreshold && metrics.Node.MemPercent < NodeMemThreshold
}

func (m *MultiLevelPolicy) podResourceLTThreshold(cpu, mem float64) bool {
	return cpu < PodCpuThreshold && mem < PodMemThreshold
}

func (m *MultiLevelPolicy) localNodeResourceLTThreshold() bool {
	return m.nodeResourceLTThreshold(m.selfName)
}

func (m *MultiLevelPolicy) localPodResourceLTThreshold(appName string) bool {
	logHeader := "[MultiLevelPolicy.localPodResourceLTThreshold]"
	app := m.store.Metrics[m.selfName].Node.Apps[appName]
	if app == nil {
		klog.Errorf("%s the metrics for app %s can't be found", logHeader, appName)
		return false
	}
	return app.CpuAvgPercent < PodCpuThreshold && app.MemAvgPercent < PodMemThreshold
}

func (m *MultiLevelPolicy) pickLocal(appName string) string {
	logHeader := "[MultiLevelPolicy.pickLocal]"
	klog.Infof("%s pick app %s pod in local", logHeader, appName)
	var pod monitor.PodInfo
	app := m.store.Metrics[m.selfName].Node.Apps[appName]
	for {
		pod = app.Pods[app.Index]
		app.Index = (app.Index + 1) % len(app.Pods)
		if m.podResourceLTThreshold(pod.CpuPercent, pod.MemPercent) {
			break
		}
	}
	klog.Infof("%s pick app %s endpoint: %s", logHeader, appName, pod.Endpoint)
	return pod.Endpoint
}

func (m *MultiLevelPolicy) inClusterNodeResourceLTThreshold() bool {
	return m.store.InCluster.NodeCpuAvgPercent < NodeCpuThreshold &&
		m.store.InCluster.NodeMemAvgPercent < NodeMemThreshold
}

func (m *MultiLevelPolicy) inClusterPodResourceLTThreshold(appName string) bool {
	logHeader := "[MultiLevelPolicy.inClusterPodResourceLTThreshold]"
	app, ok := m.store.InCluster.AppsAvg[appName]
	if !ok {
		klog.Errorf("%s the avg metrics for app %s can't be found", logHeader, appName)
		return false
	}
	return app.CpuAvgPercent < PodCpuThreshold && app.MemAvgPercent < PodCpuThreshold
}

func (m *MultiLevelPolicy) pickInCluster(appName string) string {
	logHeader := "[MultiLevelPolicy.pickInCluster]"
	klog.Infof("%s pick app %s pod in cluster", logHeader, appName)
	for {
		for _, node := range m.store.InCluster.Nodes {
			if node.Name == m.selfName {
				continue
			}
			if !m.nodeResourceLTThreshold(node.Name) {
				continue
			}
			if _, exist := node.Apps[appName]; !exist {
				continue
			}
			app := node.Apps[appName]
			for {
				pod := app.Pods[app.Index]
				app.Index = (app.Index + 1) % len(app.Pods)
				if m.podResourceLTThreshold(pod.CpuPercent, pod.MemPercent) {
					klog.Infof("%s pick app %s endpoint: %s", logHeader, appName, pod.Endpoint)
					return pod.Endpoint
				}
				if app.Index == 0 {
					break
				}
			}
		}
	}
}

func (m *MultiLevelPolicy) pickOutCluster(appName string) string {
	logHeader := "[MultiLevelPolicy.pickOutCluster]"
	klog.Infof("%s pick app %s pod in out cluster", logHeader, appName)
	var pod monitor.PodInfo
	if m.store.OutCluster == nil || m.store.OutCluster.BestNode == nil || m.store.OutCluster.BestNode[appName] == nil {
		klog.Errorf("%s the app %s in out cluster can't be found", logHeader, appName)
		return ""
	}
	app := m.store.OutCluster.BestNode[appName].Apps[appName]
	for {
		pod = app.Pods[app.Index]
		app.Index = (app.Index + 1) % len(app.Pods)
		if m.podResourceLTThreshold(pod.CpuPercent, pod.MemPercent) {
			break
		}
	}
	klog.Infof("%s pick app %s endpoint: %s", logHeader, appName, pod.Endpoint)
	return pod.Endpoint
}

func (m *MultiLevelPolicy) localExistAppPods(appName string) bool {
	localMetrics := m.store.Metrics[m.selfName]
	if localMetrics == nil || localMetrics.Status != "OK" || localMetrics.Node == nil {
		return false
	}
	if localMetrics.Node.Apps != nil && localMetrics.Node.Apps[appName] != nil {
		return true
	}
	return false
}

func (m *MultiLevelPolicy) inClusterExistAppPods(appName string) bool {
	if _, exist := m.store.InCluster.AppsAvg[appName]; exist {
		return true
	}
	return false
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
