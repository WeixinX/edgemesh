package monitor

var (
	heartbeatMsg = `
{
  "status": "OK",
  "node": {
    "name": "ke-worker2",
    "cluster": "Nanjing",
    "cpu": 20,
    "mem": 30
  },
  "apps": [
    {
      "name": "hostname-lb-edge",
      "port": "9376",
      "pods": [
        { 
          "name": "hostname-lb-edge-666597969d-jjwpf",
          "ip": "10.244.2.7",
          "cpu": 30, 
          "mem": 20
        },
        { 
          "name": "hostname-lb-edge-666597969d-jnq8v",
          "ip": "10.244.2.6",
          "cpu": 35,
          "mem": 25
        }
      ]
    }
  ]
}`

	// expectedMetricsItem = &MetricsItem{
	// 	Status: "OK",
	// 	Node: &NodeInfo{
	// 		Name:        "ke-worker2",
	// 		ClusterName: "Nanjing",
	// 		CpuUsage:    20,
	// 		MemUsage:    30,
	// 	},
	// 	Apps: []AppInfo{
	// 		{
	// 			Name: "hostname-lb-edge",
	// 			Port: "9376",
	// 			Pods: []PodInfo{
	// 				{
	// 					Name:     "hostname-lb-edge-666597969d-jjwpf",
	// 					IP:       "10.244.2.7",
	// 					CpuUsage: 30,
	// 					MemUsage: 20,
	// 				},
	// 				{
	// 					Name:     "hostname-lb-edge-666597969d-jnq8v",
	// 					IP:       "10.244.2.6",
	// 					CpuUsage: 35,
	// 					MemUsage: 25,
	// 				},
	// 			},
	// 		},
	// 	},
	// }

	stateJson = `
{
  "id": "6a825448c5ef07ac0c3b99532b18bdec0e1d0bb1eb41b4672d570634a1409a85",
  "init_process_pid": 232402,
  "init_process_start": 49729757,
  "created": "2023-11-23T08:32:23.988701148Z",
  "config": {
    "no_pivot_root": false,
    "parent_death_signal": 0,
    "rootfs": "/run/containerd/io.containerd.runtime.v2.task/k8s.io/6a825448c5ef07ac0c3b99532b18bdec0e1d0bb1eb41b4672d570634a1409a85/rootfs",
    "umask": null,
    "readonlyfs": false,
    "rootPropagation": 0,
    "mounts": [
      {
        "source": "proc",
        "destination": "/proc",
        "device": "proc",
        "flags": 14,
        "propagation_flags": null,
        "data": "",
        "relabel": "",
        "rec_attr": null,
        "extensions": 0,
        "premount_cmds": null,
        "postmount_cmds": null
      },
      {
        "source": "tmpfs",
        "destination": "/dev",
        "device": "tmpfs",
        "flags": 16777218,
        "propagation_flags": null,
        "data": "mode=755,size=65536k",
        "relabel": "",
        "rec_attr": null,
        "extensions": 0,
        "premount_cmds": null,
        "postmount_cmds": null
      },
      {
        "source": "devpts",
        "destination": "/dev/pts",
        "device": "devpts",
        "flags": 10,
        "propagation_flags": null,
        "data": "newinstance,ptmxmode=0666,mode=0620,gid=5",
        "relabel": "",
        "rec_attr": null,
        "extensions": 0,
        "premount_cmds": null,
        "postmount_cmds": null
      },
      {
        "source": "mqueue",
        "destination": "/dev/mqueue",
        "device": "mqueue",
        "flags": 14,
        "propagation_flags": null,
        "data": "",
        "relabel": "",
        "rec_attr": null,
        "extensions": 0,
        "premount_cmds": null,
        "postmount_cmds": null
      },
      {
        "source": "sysfs",
        "destination": "/sys",
        "device": "sysfs",
        "flags": 15,
        "propagation_flags": null,
        "data": "",
        "relabel": "",
        "rec_attr": null,
        "extensions": 0,
        "premount_cmds": null,
        "postmount_cmds": null
      },
      {
        "source": "cgroup",
        "destination": "/sys/fs/cgroup",
        "device": "cgroup",
        "flags": 2097167,
        "propagation_flags": null,
        "data": "",
        "relabel": "",
        "rec_attr": null,
        "extensions": 0,
        "premount_cmds": null,
        "postmount_cmds": null
      },
      {
        "source": "/var/lib/edged/pods/005f082b-5fd9-4650-8ef4-473e63bc2d13/etc-hosts",
        "destination": "/etc/hosts",
        "device": "bind",
        "flags": 20480,
        "propagation_flags": [
          278528
        ],
        "data": "",
        "relabel": "",
        "rec_attr": null,
        "extensions": 0,
        "premount_cmds": null,
        "postmount_cmds": null
      },
      {
        "source": "/var/lib/edged/pods/005f082b-5fd9-4650-8ef4-473e63bc2d13/containers/hostname/92ee6a3a",
        "destination": "/dev/termination-log",
        "device": "bind",
        "flags": 20480,
        "propagation_flags": [
          278528
        ],
        "data": "",
        "relabel": "",
        "rec_attr": null,
        "extensions": 0,
        "premount_cmds": null,
        "postmount_cmds": null
      },
      {
        "source": "/var/lib/containerd/io.containerd.grpc.v1.cri/sandboxes/446c65ab43d490ab8d7f693a46050db88f66a2bf657eda0c3de8bc8bab516be4/hostname",
        "destination": "/etc/hostname",
        "device": "bind",
        "flags": 20480,
        "propagation_flags": [
          278528
        ],
        "data": "",
        "relabel": "",
        "rec_attr": null,
        "extensions": 0,
        "premount_cmds": null,
        "postmount_cmds": null
      },
      {
        "source": "/var/lib/containerd/io.containerd.grpc.v1.cri/sandboxes/446c65ab43d490ab8d7f693a46050db88f66a2bf657eda0c3de8bc8bab516be4/resolv.conf",
        "destination": "/etc/resolv.conf",
        "device": "bind",
        "flags": 20480,
        "propagation_flags": [
          278528
        ],
        "data": "",
        "relabel": "",
        "rec_attr": null,
        "extensions": 0,
        "premount_cmds": null,
        "postmount_cmds": null
      },
      {
        "source": "/run/containerd/io.containerd.grpc.v1.cri/sandboxes/446c65ab43d490ab8d7f693a46050db88f66a2bf657eda0c3de8bc8bab516be4/shm",
        "destination": "/dev/shm",
        "device": "bind",
        "flags": 20480,
        "propagation_flags": [
          278528
        ],
        "data": "",
        "relabel": "",
        "rec_attr": null,
        "extensions": 0,
        "premount_cmds": null,
        "postmount_cmds": null
      },
      {
        "source": "/var/lib/edged/pods/005f082b-5fd9-4650-8ef4-473e63bc2d13/volumes/kubernetes.io~projected/kube-api-access-9j2qj",
        "destination": "/var/run/secrets/kubernetes.io/serviceaccount",
        "device": "bind",
        "flags": 20481,
        "propagation_flags": [
          278528
        ],
        "data": "",
        "relabel": "",
        "rec_attr": null,
        "extensions": 0,
        "premount_cmds": null,
        "postmount_cmds": null
      }
    ],
    "devices": [
      {
        "type": 99,
        "major": 1,
        "minor": 3,
        "permissions": "rwm",
        "allow": true,
        "path": "/dev/null",
        "file_mode": 438,
        "uid": 0,
        "gid": 0
      },
      {
        "type": 99,
        "major": 1,
        "minor": 8,
        "permissions": "rwm",
        "allow": true,
        "path": "/dev/random",
        "file_mode": 438,
        "uid": 0,
        "gid": 0
      },
      {
        "type": 99,
        "major": 1,
        "minor": 7,
        "permissions": "rwm",
        "allow": true,
        "path": "/dev/full",
        "file_mode": 438,
        "uid": 0,
        "gid": 0
      },
      {
        "type": 99,
        "major": 5,
        "minor": 0,
        "permissions": "rwm",
        "allow": true,
        "path": "/dev/tty",
        "file_mode": 438,
        "uid": 0,
        "gid": 0
      },
      {
        "type": 99,
        "major": 1,
        "minor": 5,
        "permissions": "rwm",
        "allow": true,
        "path": "/dev/zero",
        "file_mode": 438,
        "uid": 0,
        "gid": 0
      },
      {
        "type": 99,
        "major": 1,
        "minor": 9,
        "permissions": "rwm",
        "allow": true,
        "path": "/dev/urandom",
        "file_mode": 438,
        "uid": 0,
        "gid": 0
      }
    ],
    "mount_label": "",
    "hostname": "",
    "namespaces": [
      {
        "type": "NEWPID",
        "path": ""
      },
      {
        "type": "NEWIPC",
        "path": "/proc/232296/ns/ipc"
      },
      {
        "type": "NEWUTS",
        "path": "/proc/232296/ns/uts"
      },
      {
        "type": "NEWNS",
        "path": ""
      },
      {
        "type": "NEWNET",
        "path": "/proc/232296/ns/net"
      }
    ],
    "capabilities": {
      "Bounding": [
        "CAP_CHOWN",
        "CAP_DAC_OVERRIDE",
        "CAP_FSETID",
        "CAP_FOWNER",
        "CAP_MKNOD",
        "CAP_NET_RAW",
        "CAP_SETGID",
        "CAP_SETUID",
        "CAP_SETFCAP",
        "CAP_SETPCAP",
        "CAP_NET_BIND_SERVICE",
        "CAP_SYS_CHROOT",
        "CAP_KILL",
        "CAP_AUDIT_WRITE"
      ],
      "Effective": [
        "CAP_CHOWN",
        "CAP_DAC_OVERRIDE",
        "CAP_FSETID",
        "CAP_FOWNER",
        "CAP_MKNOD",
        "CAP_NET_RAW",
        "CAP_SETGID",
        "CAP_SETUID",
        "CAP_SETFCAP",
        "CAP_SETPCAP",
        "CAP_NET_BIND_SERVICE",
        "CAP_SYS_CHROOT",
        "CAP_KILL",
        "CAP_AUDIT_WRITE"
      ],
      "Inheritable": null,
      "Permitted": [
        "CAP_CHOWN",
        "CAP_DAC_OVERRIDE",
        "CAP_FSETID",
        "CAP_FOWNER",
        "CAP_MKNOD",
        "CAP_NET_RAW",
        "CAP_SETGID",
        "CAP_SETUID",
        "CAP_SETFCAP",
        "CAP_SETPCAP",
        "CAP_NET_BIND_SERVICE",
        "CAP_SYS_CHROOT",
        "CAP_KILL",
        "CAP_AUDIT_WRITE"
      ],
      "Ambient": null
    },
    "networks": null,
    "routes": null,
    "cgroups": {
      "path": "/kubepods/pod005f082b-5fd9-4650-8ef4-473e63bc2d13/6a825448c5ef07ac0c3b99532b18bdec0e1d0bb1eb41b4672d570634a1409a85",
      "scope_prefix": "",
      "devices": [
        {
          "type": 97,
          "major": -1,
          "minor": -1,
          "permissions": "rwm",
          "allow": false
        },
        {
          "type": 99,
          "major": -1,
          "minor": -1,
          "permissions": "m",
          "allow": true
        },
        {
          "type": 98,
          "major": -1,
          "minor": -1,
          "permissions": "m",
          "allow": true
        },
        {
          "type": 99,
          "major": 1,
          "minor": 3,
          "permissions": "rwm",
          "allow": true
        },
        {
          "type": 99,
          "major": 1,
          "minor": 8,
          "permissions": "rwm",
          "allow": true
        },
        {
          "type": 99,
          "major": 1,
          "minor": 7,
          "permissions": "rwm",
          "allow": true
        },
        {
          "type": 99,
          "major": 5,
          "minor": 0,
          "permissions": "rwm",
          "allow": true
        },
        {
          "type": 99,
          "major": 1,
          "minor": 5,
          "permissions": "rwm",
          "allow": true
        },
        {
          "type": 99,
          "major": 1,
          "minor": 9,
          "permissions": "rwm",
          "allow": true
        },
        {
          "type": 99,
          "major": 136,
          "minor": -1,
          "permissions": "rwm",
          "allow": true
        },
        {
          "type": 99,
          "major": 5,
          "minor": 2,
          "permissions": "rwm",
          "allow": true
        },
        {
          "type": 99,
          "major": 10,
          "minor": 200,
          "permissions": "rwm",
          "allow": true
        }
      ],
      "memory": 134217728,
      "memory_reservation": 0,
      "memory_swap": 0,
      "cpu_shares": 102,
      "cpu_quota": 10000,
      "cpu_period": 100000,
      "cpu_rt_quota": 0,
      "cpu_rt_period": 0,
      "cpuset_cpus": "",
      "cpuset_mems": "",
      "pids_limit": 0,
      "blkio_weight": 0,
      "blkio_leaf_weight": 0,
      "blkio_weight_device": null,
      "blkio_throttle_read_bps_device": null,
      "blkio_throttle_write_bps_device": null,
      "blkio_throttle_read_iops_device": null,
      "blkio_throttle_write_iops_device": null,
      "freezer": "",
      "hugetlb_limit": null,
      "oom_kill_disable": false,
      "memory_swappiness": null,
      "net_prio_ifpriomap": null,
      "net_cls_classid_u": 0,
      "rdma": null,
      "cpu_weight": 4,
      "unified": null,
      "Systemd": false,
      "Rootless": false
    },
    "oom_score_adj": -997,
    "uid_mappings": null,
    "gid_mappings": null,
    "mask_paths": [
      "/proc/acpi",
      "/proc/kcore",
      "/proc/keys",
      "/proc/latency_stats",
      "/proc/timer_list",
      "/proc/timer_stats",
      "/proc/sched_debug",
      "/proc/scsi",
      "/sys/firmware"
    ],
    "readonly_paths": [
      "/proc/asound",
      "/proc/bus",
      "/proc/fs",
      "/proc/irq",
      "/proc/sys",
      "/proc/sysrq-trigger"
    ],
    "sysctl": null,
    "seccomp": null,
    "Hooks": {
      "createContainer": null,
      "createRuntime": null,
      "poststart": null,
      "poststop": null,
      "prestart": null,
      "startContainer": null
    },
    "version": "1.0.2-dev",
    "labels": [
      "io.kubernetes.cri.sandbox-uid=005f082b-5fd9-4650-8ef4-473e63bc2d13",
      "io.kubernetes.cri.container-name=hostname",
      "io.kubernetes.cri.container-type=container",
      "io.kubernetes.cri.image-name=docker.io/poorunga/serve_hostname:latest",
      "io.kubernetes.cri.sandbox-id=446c65ab43d490ab8d7f693a46050db88f66a2bf657eda0c3de8bc8bab516be4",
      "io.kubernetes.cri.sandbox-name=hostname-lb-edge-64df67bd8b-xmzhv",
      "io.kubernetes.cri.sandbox-namespace=default",
      "bundle=/run/containerd/io.containerd.runtime.v2.task/k8s.io/6a825448c5ef07ac0c3b99532b18bdec0e1d0bb1eb41b4672d570634a1409a85"
    ],
    "no_new_keyring": false
  },
  "rootless": false,
  "cgroup_paths": {
    "": "/sys/fs/cgroup/unified/kubepods/pod005f082b-5fd9-4650-8ef4-473e63bc2d13/6a825448c5ef07ac0c3b99532b18bdec0e1d0bb1eb41b4672d570634a1409a85",
    "blkio": "/sys/fs/cgroup/blkio/kubepods/pod005f082b-5fd9-4650-8ef4-473e63bc2d13/6a825448c5ef07ac0c3b99532b18bdec0e1d0bb1eb41b4672d570634a1409a85",
    "cpu": "/sys/fs/cgroup/cpu,cpuacct/kubepods/pod005f082b-5fd9-4650-8ef4-473e63bc2d13/6a825448c5ef07ac0c3b99532b18bdec0e1d0bb1eb41b4672d570634a1409a85",
    "cpuacct": "/sys/fs/cgroup/cpu,cpuacct/kubepods/pod005f082b-5fd9-4650-8ef4-473e63bc2d13/6a825448c5ef07ac0c3b99532b18bdec0e1d0bb1eb41b4672d570634a1409a85",
    "cpuset": "/sys/fs/cgroup/cpuset/kubepods/pod005f082b-5fd9-4650-8ef4-473e63bc2d13/6a825448c5ef07ac0c3b99532b18bdec0e1d0bb1eb41b4672d570634a1409a85",
    "devices": "/sys/fs/cgroup/devices/kubepods/pod005f082b-5fd9-4650-8ef4-473e63bc2d13/6a825448c5ef07ac0c3b99532b18bdec0e1d0bb1eb41b4672d570634a1409a85",
    "freezer": "/sys/fs/cgroup/freezer/kubepods/pod005f082b-5fd9-4650-8ef4-473e63bc2d13/6a825448c5ef07ac0c3b99532b18bdec0e1d0bb1eb41b4672d570634a1409a85",
    "hugetlb": "/sys/fs/cgroup/hugetlb/kubepods/pod005f082b-5fd9-4650-8ef4-473e63bc2d13/6a825448c5ef07ac0c3b99532b18bdec0e1d0bb1eb41b4672d570634a1409a85",
    "memory": "/sys/fs/cgroup/memory/kubepods/pod005f082b-5fd9-4650-8ef4-473e63bc2d13/6a825448c5ef07ac0c3b99532b18bdec0e1d0bb1eb41b4672d570634a1409a85",
    "name=systemd": "/sys/fs/cgroup/systemd/kubepods/pod005f082b-5fd9-4650-8ef4-473e63bc2d13/6a825448c5ef07ac0c3b99532b18bdec0e1d0bb1eb41b4672d570634a1409a85",
    "net_cls": "/sys/fs/cgroup/net_cls,net_prio/kubepods/pod005f082b-5fd9-4650-8ef4-473e63bc2d13/6a825448c5ef07ac0c3b99532b18bdec0e1d0bb1eb41b4672d570634a1409a85",
    "net_prio": "/sys/fs/cgroup/net_cls,net_prio/kubepods/pod005f082b-5fd9-4650-8ef4-473e63bc2d13/6a825448c5ef07ac0c3b99532b18bdec0e1d0bb1eb41b4672d570634a1409a85",
    "perf_event": "/sys/fs/cgroup/perf_event/kubepods/pod005f082b-5fd9-4650-8ef4-473e63bc2d13/6a825448c5ef07ac0c3b99532b18bdec0e1d0bb1eb41b4672d570634a1409a85",
    "pids": "/sys/fs/cgroup/pids/kubepods/pod005f082b-5fd9-4650-8ef4-473e63bc2d13/6a825448c5ef07ac0c3b99532b18bdec0e1d0bb1eb41b4672d570634a1409a85",
    "rdma": "/sys/fs/cgroup/rdma/kubepods/pod005f082b-5fd9-4650-8ef4-473e63bc2d13/6a825448c5ef07ac0c3b99532b18bdec0e1d0bb1eb41b4672d570634a1409a85"
  },
  "namespace_paths": {
    "NEWCGROUP": "/proc/232402/ns/cgroup",
    "NEWIPC": "/proc/232402/ns/ipc",
    "NEWNET": "/proc/232402/ns/net",
    "NEWNS": "/proc/232402/ns/mnt",
    "NEWPID": "/proc/232402/ns/pid",
    "NEWUSER": "/proc/232402/ns/user",
    "NEWUTS": "/proc/232402/ns/uts"
  },
  "external_descriptors": [
    "/dev/null",
    "pipe:[2683570]",
    "pipe:[2683571]"
  ],
  "intel_rdt_path": ""
}`

	specJson = `
{
  "/kubepods/pod005f082b-5fd9-4650-8ef4-473e63bc2d13/6a825448c5ef07ac0c3b99532b18bdec0e1d0bb1eb41b4672d570634a1409a85": {
    "creation_time": "2023-11-23T16:32:23.851714936+08:00",
    "aliases": [
      "6a825448c5ef07ac0c3b99532b18bdec0e1d0bb1eb41b4672d570634a1409a85",
      "/kubepods/pod005f082b-5fd9-4650-8ef4-473e63bc2d13/6a825448c5ef07ac0c3b99532b18bdec0e1d0bb1eb41b4672d570634a1409a85"
    ],
    "namespace": "containerd",
    "labels": {
      "io.cri-containerd.kind": "container",
      "io.kubernetes.container.name": "hostname",
      "io.kubernetes.pod.name": "hostname-lb-edge-64df67bd8b-xmzhv",
      "io.kubernetes.pod.namespace": "default",
      "io.kubernetes.pod.uid": "005f082b-5fd9-4650-8ef4-473e63bc2d13"
    },
    "has_cpu": true,
    "cpu": {
      "limit": 102,
      "max_limit": 0
    },
    "has_memory": true,
    "memory": {
      "limit": 134217728,
      "reservation": 9223372036854772000
    },
    "has_hugetlb": false,
    "has_custom_metrics": false,
    "has_processes": false,
    "processes": {},
    "has_network": false,
    "has_filesystem": false,
    "has_diskio": true,
    "image": "docker.io/poorunga/serve_hostname:latest"
  }
}`

	summaryJson = `
{
  "/kubepods/pod005f082b-5fd9-4650-8ef4-473e63bc2d13/6a825448c5ef07ac0c3b99532b18bdec0e1d0bb1eb41b4672d570634a1409a85": {
    "timestamp": "2023-11-23T22:33:19.11100175+08:00",
    "latest_usage": {
      "cpu": 0,
      "memory": 3416064
    },
    "minute_usage": {
      "percent_complete": 100,
      "cpu": {
        "present": true,
        "mean": 0,
        "max": 0,
        "fifty": 0,
        "ninety": 0,
        "ninetyfive": 0
      },
      "memory": {
        "present": true,
        "mean": 3416064,
        "max": 3416064,
        "fifty": 3416064,
        "ninety": 3416064,
        "ninetyfive": 3416064
      }
    },
    "hour_usage": {
      "percent_complete": 18,
      "cpu": {
        "present": true,
        "mean": 0,
        "max": 0,
        "fifty": 0,
        "ninety": 0,
        "ninetyfive": 0
      },
      "memory": {
        "present": true,
        "mean": 3416064,
        "max": 3416064,
        "fifty": 3416064,
        "ninety": 3416064,
        "ninetyfive": 3416064
      }
    },
    "day_usage": {
      "percent_complete": 0,
      "cpu": {
        "present": true,
        "mean": 0,
        "max": 0,
        "fifty": 0,
        "ninety": 0,
        "ninetyfive": 0
      },
      "memory": {
        "present": true,
        "mean": 3416064,
        "max": 3416064,
        "fifty": 3416064,
        "ninety": 3416064,
        "ninetyfive": 3416064
      }
    }
  }
}`
)
