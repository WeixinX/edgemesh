package monitor

import (
	"bufio"
	"fmt"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/kubeedge/edgemesh/pkg/tunnel/pb/heartbeat"

	"github.com/fsnotify/fsnotify"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
	"k8s.io/klog/v2"
)

const (
	watchRetryNum          = 3
	watchRetryInterval     = 2 * time.Second
	watchContainerInterval = 2 * time.Second
	watchNodeInterval      = 2 * time.Second

	updateToStoreInterval = 2 * time.Second
	resetStatusInterval   = 5 * time.Second

	errIsSandbox     = "is sandbox"
	errInvalidIpFile = "invalid ip file"
)

type ContainerState struct {
	ContainerID        string
	CgroupsPath        string
	IsSandbox          bool
	SandboxID          string
	SandboxName        string
	SandboxNamespace   string
	IP, Port           string
	CpuLimit, MemLimit float64
	CpuUsage, MemUsage float64
	lockUsage          sync.RWMutex // protects CpuUsage & MemUsage
	stop               chan struct{}
}

func (s *ContainerState) String() string {
	s.lockUsage.RLock()
	defer s.lockUsage.RUnlock()
	return fmt.Sprintf("cgroups path: %s\n"+
		"is sandbox: %v\n"+
		"sandbox-id: %s\n"+
		"sandbox-name: %s\n"+
		"sandbox-namespace: %s\n"+
		"ip:port: %s:%s\n"+
		"cpu-percent: %.2f/%.2f=%.2f\n"+
		"mem-percent: %.2f/%.2f=%.2f",
		s.CgroupsPath, s.IsSandbox, s.SandboxID, s.SandboxName, s.SandboxNamespace, s.IP, s.Port,
		s.CpuUsage, s.CpuLimit, s.CpuUsage/s.CpuLimit,
		s.MemUsage, s.MemLimit, s.MemUsage/s.MemLimit,
	)
}

func (s *ContainerState) watchUsage(cli *CAdvisorClient) {
	logHeader := "[ContainerState.watchUsage]"
	ticker := time.NewTicker(watchContainerInterval)
	defer ticker.Stop()
	for {
		select {
		case <-s.stop:
			return
		case <-ticker.C:
			cpuUsage, memUsage, err := getContainerResourceUsage(cli, s.CgroupsPath)
			if err != nil {
				klog.Errorf("%s failed to get container resource usage: %v", logHeader, err)
				continue
			}
			s.lockUsage.Lock()
			s.CpuUsage = cpuUsage
			s.MemUsage = memUsage
			s.lockUsage.Unlock()
			// klog.Infof("%s watch container resource usage: %s", logHeader, s.String())
		}
	}
}

func (s *ContainerState) stopWatchUsage() {
	close(s.stop)
}

type LocalMonitor struct {
	client                *CAdvisorClient
	store                 *MetricsStore
	node                  *NodeState
	nLock                 sync.RWMutex
	containerStates       map[string]*ContainerState // ContainerID -> ContainerState
	csLock                sync.RWMutex               // protects containerStates
	sandboxIPs            map[string]string          // SandboxID -> SandboxIP
	sandboxIp2Id          map[string]string
	ipLock                sync.RWMutex // protects sandboxIPs & sandboxIp2Id
	containerStateEventCh chan ContainerStateEvent
	sandboxIpEventCh      chan SandboxIpEvent
	ipBindEventCh         chan IpBindEvent
	stop                  chan struct{}
}

func NewLocalMonitor(cAdvisorAddr string, store *MetricsStore) *LocalMonitor {
	m := &LocalMonitor{
		client:                NewCAdvisorClient(cAdvisorAddr),
		store:                 store,
		node:                  new(NodeState),
		nLock:                 sync.RWMutex{},
		containerStates:       make(map[string]*ContainerState),
		csLock:                sync.RWMutex{},
		sandboxIPs:            make(map[string]string),
		sandboxIp2Id:          make(map[string]string),
		ipLock:                sync.RWMutex{},
		containerStateEventCh: make(chan ContainerStateEvent, 10),
		sandboxIpEventCh:      make(chan SandboxIpEvent, 10),
		ipBindEventCh:         make(chan IpBindEvent, 10),
		stop:                  make(chan struct{}),
	}
	klog.Info("Initializing the local monitor...")
	m.traversalSandboxIpDir()
	m.traversalContainerStateDir()
	return m
}

func (m *LocalMonitor) Run() {
	klog.Info("Local monitor is running...")
	go m.watchContainerStateDir(m.containerStateEventCh)
	go m.handleContainerStateEvent(m.containerStateEventCh)

	go m.watchSandboxIpDir(m.sandboxIpEventCh)
	go m.handleSandboxIpEvent(m.sandboxIpEventCh)

	go m.ipBindLoop()

	go m.watchNodeState()
	go m.updateToStoreLoop()

	<-m.stop
}

func (m *LocalMonitor) Stop() {
	klog.Info("Stopping the local monitor...")
	close(m.stop)
	m.csLock.Lock()
	for _, state := range m.containerStates {
		state.stopWatchUsage()
	}
	m.csLock.Unlock()
}

func (m *LocalMonitor) getContainerState(containerID string) (*ContainerState, error) {
	// 获取容器的 state.json 信息
	isSandbox, cgroupsPath, sandboxID, sandboxName, sandboxNamespace, err := parseContainerState(containerID)
	if isSandbox {
		return nil, fmt.Errorf(errIsSandbox)
	}
	if err != nil {
		return nil, err
	}

	// 获取 sandbox 的 state.json 信息
	sandboxCgroupsPath, err := parseSandboxState(sandboxID)
	if err != nil {
		return nil, err
	}

	// 获取自定义的 sandbox labels 信息
	port, err := getSandboxLabelPort(m.client, sandboxCgroupsPath)
	if err != nil {
		return nil, err
	}

	// 获取容器的 cpuLimit、memLimit
	cpuLimit, memLimit, err := getContainerResourceLimit(m.client, cgroupsPath)
	if err != nil {
		return nil, err
	}

	// 获取容器的 cpuUsage、memUsage
	cpuUsage, memUsage, err := getContainerResourceUsage(m.client, cgroupsPath)
	if err != nil {
		return nil, err
	}

	c := &ContainerState{
		ContainerID:      containerID,
		CgroupsPath:      cgroupsPath,
		IsSandbox:        false,
		SandboxID:        sandboxID,
		SandboxName:      sandboxName,
		SandboxNamespace: sandboxNamespace,
		Port:             port,
		CpuUsage:         cpuUsage,
		MemUsage:         memUsage,
		CpuLimit:         cpuLimit,
		MemLimit:         memLimit,
		lockUsage:        sync.RWMutex{},
		stop:             make(chan struct{}),
	}
	go c.watchUsage(m.client)
	return c, nil
}

func (m *LocalMonitor) traversalContainerStateDir() {
	logHeader := "[LocalMonitor.traversalContainerStateDir]"

	m.csLock.Lock()
	defer m.csLock.Unlock()
	err := filepath.WalkDir(containerStateDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() || path == containerStateDir {
			return nil
		}

		state, err := m.getContainerState(d.Name())
		if err != nil {
			if err.Error() == errIsSandbox {
				return nil
			}
			klog.Errorf("%s failed to get container state: %v", logHeader, err)
			return nil
		}

		if state.ContainerID != "" && state.SandboxID != "" {
			klog.Info("%s get a container state %s", logHeader, state.ContainerID)
			m.containerStates[state.ContainerID] = state
			m.ipBindEventCh <- IpBindEvent{
				ContainerID: state.ContainerID,
				SandboxID:   state.SandboxID,
			}
		}
		return nil
	})
	if err != nil {
		klog.Fatalf("%s failed to traversal container state dir: %v", logHeader, err)
	}
}

func (m *LocalMonitor) traversalSandboxIpDir() {
	logHeader := "[LocalMonitor.traversalSandboxIpDir]"

	m.ipLock.Lock()
	defer m.ipLock.Unlock()
	err := filepath.WalkDir(sandboxIpDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		sandboxID, sandboxIP, err := getSandboxIP(d.Name())
		if err != nil {
			if err.Error() == errInvalidIpFile {
				return nil
			}
			klog.Errorf("%s failed to get sandbox ip: %v", logHeader, err)
			return nil
		}

		klog.Infof("%s get a id-ip pair: id %s -> ip %s", logHeader, sandboxID, sandboxIP)
		m.sandboxIPs[sandboxID] = sandboxIP
		m.sandboxIp2Id[sandboxIP] = sandboxID
		return nil
	})
	if err != nil {
		klog.Fatalf("%s failed to traversal sandbox ip dir: %v", logHeader, err)
	}
}

type EventType string

const (
	EventTypeAdd EventType = "Add"
	EventTypeDel EventType = "Del"
)

type ContainerStateEvent struct {
	Type        EventType
	ContainerID string
}

// TODO(WeixinX): 添加一个定期巡检（traversal）的功能，向对应的 channel 发送事件，避免在容器创建时可能会丢失事件

func (m *LocalMonitor) watchContainerStateDir(eventCh chan<- ContainerStateEvent) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		klog.Fatalf("failed to new container state watcher: %v", err)
	}
	defer watcher.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)

		for {
			select {
			case <-m.stop:
				return
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Op&fsnotify.Create != 0 {
					eventCh <- ContainerStateEvent{
						Type:        EventTypeAdd,
						ContainerID: filepath.Base(event.Name),
					}
				} else if event.Op&fsnotify.Remove != 0 {
					eventCh <- ContainerStateEvent{
						Type:        EventTypeDel,
						ContainerID: filepath.Base(event.Name),
					}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					klog.Errorf("container state watcher error: %v", err)
					return
				}
			}
		}
	}()

	err = watcher.Add(containerStateDir)
	if err != nil {
		klog.Fatalf("failed to add container state watch path: %v", err)
	}
	klog.Info("started watching container state...")
	<-done
}

func (m *LocalMonitor) handleContainerStateEvent(eventCh <-chan ContainerStateEvent) {
	for {
		select {
		case <-m.stop:
			return
		case event, ok := <-eventCh:
			if !ok {
				return
			}
			switch event.Type {
			case EventTypeAdd:
				klog.Infof("handle add container event: %+v", event)
				var (
					state *ContainerState
					err   error
				)
				for i := 0; i < watchRetryNum; i++ {
					state, err = m.getContainerState(event.ContainerID)
					if err != nil && err.Error() != errIsSandbox {
						klog.Warningf("failed to get container state: %v", err)
						time.Sleep(watchRetryInterval)
						continue
					}
					break
				}
				if state == nil {
					continue
				}

				m.csLock.Lock()
				if state.ContainerID != "" && state.SandboxID != "" {
					m.containerStates[state.ContainerID] = state
					m.ipBindEventCh <- IpBindEvent{
						ContainerID: state.ContainerID,
						SandboxID:   state.SandboxID,
					}
				}
				m.csLock.Unlock()
			case EventTypeDel:
				klog.Infof("handle delete container event: %+v", event)
				m.csLock.Lock()
				if state, exist := m.containerStates[event.ContainerID]; exist {
					state.stopWatchUsage()
					delete(m.containerStates, event.ContainerID)
				}
				m.csLock.Unlock()
			default:
				klog.Errorf("unknown container state event type: %s", event.Type)
			}
		}
	}
}

func getSandboxIP(fileName string) (string, string, error) {
	ip := net.ParseIP(fileName)
	if ip == nil { // last_reserved_ip.0 或 lock
		return "", "", fmt.Errorf(errInvalidIpFile)
	}
	f, err := os.Open(filepath.Join(sandboxIpDir, fileName))
	if err != nil {
		return "", "", err
	}
	defer f.Close()

	r := bufio.NewReader(f)
	sandboxID, _, err := r.ReadLine()
	if err != nil {
		return "", "", err
	}

	return string(sandboxID), ip.String(), nil
}

type SandboxIpEvent struct {
	Type      EventType
	SandboxIP string
}

func (m *LocalMonitor) watchSandboxIpDir(eventCh chan<- SandboxIpEvent) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		klog.Fatalf("failed to new sandbox ip watcher: %v", err)
	}
	defer watcher.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)

		for {
			select {
			case <-m.stop:
				return
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Op&fsnotify.Create != 0 {
					eventCh <- SandboxIpEvent{
						Type:      EventTypeAdd,
						SandboxIP: filepath.Base(event.Name),
					}
				} else if event.Op&fsnotify.Remove != 0 {
					eventCh <- SandboxIpEvent{
						Type:      EventTypeDel,
						SandboxIP: filepath.Base(event.Name),
					}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					klog.Errorf("sandbox ip watcher error: %v", err)
					return
				}
			}
		}
	}()

	err = watcher.Add(sandboxIpDir)
	if err != nil {
		klog.Fatalf("failed to add sandbox ip watch path: %v", err)
	}
	klog.Info("started watching sandbox ip...")
	<-done
}

func (m *LocalMonitor) handleSandboxIpEvent(eventCh <-chan SandboxIpEvent) {
	for {
		select {
		case <-m.stop:
			return
		case event, ok := <-eventCh:
			if !ok {
				return
			}
			switch event.Type {
			case EventTypeAdd:
				klog.Infof("handle add sandbox ip event: %+v", event)
				var (
					id, ip string
					err    error
				)
				for i := 0; i < watchRetryNum; i++ {
					id, ip, err = getSandboxIP(event.SandboxIP)
					if err != nil && err.Error() != errInvalidIpFile {
						klog.Warningf("failed to get sandbox ip: %v", err)
						time.Sleep(watchRetryInterval)
						continue
					}
					break
				}
				if id == "" || ip == "" {
					continue
				}

				m.ipLock.Lock()
				m.sandboxIPs[id] = ip
				m.sandboxIp2Id[ip] = id
				m.ipLock.Unlock()
			case EventTypeDel:
				klog.Infof("handle delete sandbox event: %+v", event)
				m.ipLock.Lock()
				if id, exist := m.sandboxIp2Id[event.SandboxIP]; exist {
					delete(m.sandboxIPs, id)
					delete(m.sandboxIp2Id, event.SandboxIP)
				}
				m.ipLock.Unlock()
			default:
				klog.Errorf("handle unknown sandbox ip event type: %s", event.Type)
			}
		}
	}
}

type IpBindEvent struct {
	ContainerID string
	SandboxID   string
}

func (m *LocalMonitor) ipBindLoop() {
	for {
		select {
		case <-m.stop:
			return
		case e, ok := <-m.ipBindEventCh:
			if !ok { // ch closed
				return
			}
			m.csLock.Lock()
			m.ipLock.RLock()
			if state, exist := m.containerStates[e.ContainerID]; exist && state.SandboxID == e.SandboxID {
				if ip, exist := m.sandboxIPs[e.SandboxID]; exist {
					state.IP = ip
					klog.Infof("bind container %s to sandbox ip: %s", e.ContainerID, ip)
				} else {
					m.ipBindEventCh <- e
				}
			}
			m.ipLock.RUnlock()
			m.csLock.Unlock()
		}
	}
}

type NodeState struct {
	CpuPercent float64
	MemPercent float64
}

func (m *LocalMonitor) watchNodeState() {
	logHeader := "[LocalMonitor.watchNodeState]"
	ticker := time.NewTicker(watchNodeInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.stop:
			return
		case <-ticker.C:
			cpuInfo, err := cpu.Percent(0, false)
			if err != nil || len(cpuInfo) == 0 {
				klog.Errorf("%s failed to get cpu percent: %v", logHeader, err)
				continue
			}
			cpuPercent := cpuInfo[0]

			memInfo, err := mem.VirtualMemory()
			if err != nil {
				klog.Errorf("%s failed to get mem percent: %v", logHeader, err)
				continue
			}
			memPercent := memInfo.UsedPercent

			m.nLock.Lock()
			m.node.CpuPercent = cpuPercent
			m.node.MemPercent = memPercent
			m.nLock.Unlock()
			klog.Infof("%s got cpu percent: %.2f%%, mem percent: %.2f%%", logHeader, cpuPercent, memPercent)
		}
	}
}

func (m *LocalMonitor) updateToStoreLoop() {
	self := m.store.SelfName
	cluster := m.store.ClusterName
	ticker := time.NewTicker(updateToStoreInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.stop:
			return
		case <-ticker.C:
			m.store.Lock.Lock()

			if _, exist := m.store.Metrics[self]; !exist {
				m.store.Metrics[self] = &MetricsItem{
					Node: &NodeInfo{
						Name:        self,
						ClusterName: cluster,
						Apps:        make(map[string]*AppInfo),
					},
				}
			}
			state := m.store.Metrics[self]
			state.Status = "OK"
			m.nLock.RLock()
			state.Node.CpuPercent = m.node.CpuPercent
			state.Node.MemPercent = m.node.MemPercent
			m.nLock.RUnlock()

			m.csLock.RLock()
			for _, cs := range m.containerStates {
				// 假设 Pod 是由 Deployment 创建的，因此至少有两个 "-"，如 "nginx-7f7bfc4d9f-4q9q2"
				podName := cs.SandboxName
				tmp := strings.Split(podName, "-")
				if len(tmp) < 2 {
					klog.Errorf("invalid pod name format: %s", podName)
					continue
				}
				appName := strings.Join(tmp[:len(tmp)-2], "-")
				if _, exist := state.Node.Apps[appName]; !exist {
					state.Node.Apps[appName] = &AppInfo{
						Name:     appName,
						NodeName: self,
						PodIdx:   make(map[string]int),
						Pods:     make([]PodInfo, 0),
					}
				}
				app := state.Node.Apps[appName]
				if _, exist := app.PodIdx[podName]; !exist {
					app.Pods = append(app.Pods, PodInfo{
						Name:     podName,
						NodeName: self,
						Endpoint: fmt.Sprintf("%s:%s:%s:%s", self, podName, cs.IP, cs.Port),
					})
					app.PodIdx[podName] = len(app.Pods) - 1
				}
				if cs.CpuLimit == 0 || cs.MemLimit == 0 {
					continue
				}
				app.Pods[app.PodIdx[podName]].CpuPercent = cs.CpuUsage / cs.CpuLimit
				app.Pods[app.PodIdx[podName]].MemPercent = cs.MemUsage / cs.MemLimit
			}
			m.csLock.RUnlock()

			for _, app := range state.Node.Apps {
				ttCpuPercent := 0.0
				ttMemPercent := 0.0
				for _, pod := range app.Pods {
					ttCpuPercent += pod.CpuPercent
					ttMemPercent += pod.MemPercent
				}
				if app.Pods == nil || len(app.Pods) == 0 {
					continue
				}
				app.CpuAvgPercent = ttCpuPercent / float64(len(app.Pods))
				app.MemAvgPercent = ttMemPercent / float64(len(app.Pods))
			}

			hbApps := make([]*heartbeat.AppInfo, 0, len(state.Node.Apps))
			for appName, app := range state.Node.Apps {
				hbPods := make([]*heartbeat.PodInfo, 0, len(app.Pods))
				for _, pod := range app.Pods {
					tmp := strings.Split(pod.Endpoint, ":")
					if len(tmp) != 4 {
						klog.Errorf("invalid endpoint format: %s", pod.Endpoint)
						continue
					}
					ip, port := tmp[2], tmp[3]
					hbPods = append(hbPods, &heartbeat.PodInfo{
						Name:       pod.Name,
						Ip:         ip,
						Port:       port,
						CpuPercent: pod.CpuPercent,
						MemPercent: pod.MemPercent,
					})
				}
				hbApps = append(hbApps, &heartbeat.AppInfo{
					Name:      appName,
					Namespace: "", // TODO(WeixinX)
					Pods:      hbPods,
				})
			}
			if m.store.Local == nil {
				m.store.Local = &heartbeat.Heartbeat{
					Node: &heartbeat.NodeInfo{
						Name:        self,
						ClusterName: cluster,
					},
				}
			}
			m.store.Local.Status = heartbeat.Status_OK
			m.nLock.RLock()
			m.store.Local.Node.CpuPercent = m.node.CpuPercent
			m.store.Local.Node.MemPercent = m.node.MemPercent
			m.nLock.RUnlock()
			m.store.Local.Apps = hbApps

			m.store.Lock.Unlock()
		}
	}
}
