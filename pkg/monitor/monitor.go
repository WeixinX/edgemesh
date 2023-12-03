package monitor

import (
	"sort"
	"sync"
	"time"

	"github.com/kubeedge/edgemesh/pkg/tunnel/pb/heartbeat"
)

type MetricsItem struct {
	Status string
	Node   *NodeInfo
}

type NodeInfo struct {
	Name        string
	ClusterName string
	CpuPercent  float64
	MemPercent  float64
	Apps        map[string]*AppInfo
}

type AppInfo struct {
	Name          string
	NodeName      string
	CpuAvgPercent float64
	MemAvgPercent float64
	Index         int
	PodIdx        map[string]int
	Pods          []PodInfo
}

type PodInfo struct {
	Name       string
	NodeName   string
	Endpoint   string
	CpuPercent float64
	MemPercent float64
}

type InClusterInfo struct {
	NodeCpuAvgPercent float64
	NodeMemAvgPercent float64
	AppsAvg           map[string]appAvg
	Nodes             map[string]*NodeInfo
}

type appAvg struct {
	CpuAvgPercent float64
	MemAvgPercent float64
}

type OutClusterInfo struct {
	BestNode map[string]*NodeInfo
}

type MetricsStore struct {
	Metrics    map[string]*MetricsItem // nodeName -> MetricsItem
	Local      *heartbeat.Heartbeat
	InCluster  *InClusterInfo
	OutCluster *OutClusterInfo

	SelfName    string
	ClusterName string

	Lock sync.RWMutex // protects all store, TODO(WeixinX): 调整锁粒度
	stop chan struct{}
}

func NewMetricsStore(self, cluster string) *MetricsStore {
	return &MetricsStore{
		Metrics:     map[string]*MetricsItem{},
		SelfName:    self,
		ClusterName: cluster,
		Lock:        sync.RWMutex{},
		stop:        make(chan struct{}),
	}
}

const (
	updateIOClusterInterval = 5 * time.Second
)

func (s *MetricsStore) Run() {
	go s.updateIOClusterInfoLoop()
	<-s.stop
}

func (s *MetricsStore) Stop() {
	close(s.stop)
}

func (s *MetricsStore) updateIOClusterInfoLoop() {
	ticker := time.NewTicker(updateIOClusterInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stop:
			return
		case <-ticker.C:
			s.Lock.Lock()

			appType := make(map[string]struct{})
			for _, item := range s.Metrics {
				for appName := range item.Node.Apps {
					appType[appName] = struct{}{}
				}
			}

			// in cluster
			if s.InCluster == nil {
				s.InCluster = &InClusterInfo{
					Nodes:   make(map[string]*NodeInfo),
					AppsAvg: make(map[string]appAvg),
				}
			}
			ttNodeCpu := 0.0
			ttNodeMem := 0.0
			cnt := 0
			for nodeName, item := range s.Metrics {
				if item.Status != "OK" || item.Node.ClusterName != s.ClusterName {
					continue
				}

				ttNodeCpu += item.Node.CpuPercent
				ttNodeMem += item.Node.MemPercent
				cnt++
				s.InCluster.Nodes[nodeName] = item.Node
			}
			if cnt == 0 {
				continue
			}
			s.InCluster.NodeCpuAvgPercent = ttNodeCpu / float64(cnt)
			s.InCluster.NodeMemAvgPercent = ttNodeMem / float64(cnt)
			for appName := range appType {
				ttAppCpu := 0.0
				ttAppMem := 0.0
				podCnt := 0
				for _, node := range s.InCluster.Nodes {
					if app, exist := node.Apps[appName]; exist {
						n := len(app.Pods)
						ttAppCpu += app.CpuAvgPercent * float64(n)
						ttAppMem += app.MemAvgPercent * float64(n)
						podCnt += n
					}
				}
				if podCnt == 0 {
					continue
				}
				s.InCluster.AppsAvg[appName] = appAvg{
					CpuAvgPercent: ttAppCpu / float64(podCnt),
					MemAvgPercent: ttAppMem / float64(podCnt),
				}
			}

			// out cluster
			if s.OutCluster == nil {
				s.OutCluster = &OutClusterInfo{
					BestNode: make(map[string]*NodeInfo),
				}
			}
			for appName := range appType {
				candidates := make([]*NodeInfo, 0)
				for _, item := range s.Metrics {
					if item.Status != "OK" || item.Node.ClusterName == s.ClusterName {
						continue
					}

					if _, exist := item.Node.Apps[appName]; exist {
						candidates = append(candidates, item.Node)
					}
				}
				if len(candidates) == 0 {
					continue
				}
				scoreList := make([]Score, 0, len(candidates))
				for _, c := range candidates {
					// 使用 score 刻画资源剩余情况
					score := 4 - (c.CpuPercent + c.MemPercent + c.Apps[appName].CpuAvgPercent + c.Apps[appName].MemAvgPercent)
					scoreList = append(scoreList, Score{
						score:    score,
						nodeName: c.Name,
					})
				}
				sort.Sort(byScoreDesc(scoreList))
				s.OutCluster.BestNode[appName] = s.Metrics[scoreList[0].nodeName].Node
			}

			s.Lock.Unlock()
		}
	}
}

type Score struct {
	score    float64
	nodeName string
}

type byScoreDesc []Score

func (a byScoreDesc) Len() int           { return len(a) }
func (a byScoreDesc) Less(i, j int) bool { return a[i].score > a[j].score }
func (a byScoreDesc) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
