package loadbalancer

var (
//	podInfo1 = PodInfo{
//		podName:  "hostname-lb-edge-666597969d-ft5m8",
//		endpoint: "ke-master:hostname-lb-edge-666597969d-ft5m8:10.32.0.7:9376",
//		cpuUsage: 20,
//		memUsage: 20,
//	}
//
//	podInfo2 = PodInfo{
//		podName:  "hostname-lb-edge-666597969d-jjwpf",
//		endpoint: "ke-worker2:hostname-lb-edge-666597969d-jjwpf:10.244.2.7:9376",
//		cpuUsage: 20,
//		memUsage: 20,
//	}
//
//	podInfo3 = PodInfo{
//		podName:  "hostname-lb-edge-666597969d-jnq8v",
//		endpoint: "ke-worker2:hostname-lb-edge-666597969d-jnq8v:10.244.2.6:9376",
//		cpuUsage: 20,
//		memUsage: 20,
//	}
//
//	appInfoMaster = AppInfo{
//		appName:     "hostname-lb-edge",
//		cpuAvgUsage: 20,
//		memAvgUsage: 20,
//		podInfo:     []PodInfo{podInfo1},
//		index:       0,
//	}
//
//	appInfoWorker2 = AppInfo{
//		appName:     "hostname-lb-edge",
//		cpuAvgUsage: 20,
//		memAvgUsage: 20,
//		podInfo:     []PodInfo{podInfo2, podInfo3},
//		index:       0,
//	}
//
//	nodeMaster = NodeInfo{
//		nodeName:    "ke-master",
//		clusterName: "Beijing",
//		cpuUsage:    50,
//		memUsage:    50,
//		appInfo: map[string]*AppInfo{
//			"hostname-lb-edge": &appInfoMaster,
//		},
//	}
//
//	nodeWorker2 = NodeInfo{
//		nodeName:    "ke-worker",
//		clusterName: "Nanjing",
//		cpuUsage:    50,
//		memUsage:    50,
//		appInfo: map[string]*AppInfo{
//			"hostname-lb-edge": &appInfoWorker2,
//		},
//	}
//
//	mockMultiLevelPolicy = &MultiLevelPolicy{
//		selfName:    "ke-worker2",
//		clusterName: "Nanjing",
//		metrics: &Metrics{
//			node: map[string]*NodeInfo{
//				"ke-master":  &nodeMaster,
//				"ke-worker2": &nodeWorker2,
//			},
//			inCluster: &InClusterInfo{
//				nodeCpuAvgUsage: 50,
//				nodeMemAvgUsage: 50,
//				appInfo: map[string]*AppInfo{
//					"hostname-lb-edge": &appInfoWorker2,
//				},
//			},
//			outCluster: &OutClusterInfo{
//				bestNode: map[string]*NodeInfo{
//					"hostname-lb-edge": &nodeMaster,
//				},
//			},
//		},
//		lock: &sync.RWMutex{},
//	}
)
