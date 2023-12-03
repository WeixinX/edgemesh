package monitor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tidwall/gjson"
)

const (
	containerStateDir = "/var/run/containerd/runc/k8s.io"
	sandboxIpDir      = "/var/lib/cni/networks/containerd-net"

	labelContainerType    = "io.kubernetes.cri.container-type"
	labelSandboxID        = "io.kubernetes.cri.sandbox-id"
	labelSandboxName      = "io.kubernetes.cri.sandbox-name"
	labelSandboxNamespace = "io.kubernetes.cri.sandbox-namespace"
	labelPort             = "port"

	typeContainer = "container"
	typeSandbox   = "sandbox"

	requestCAdvisorTimeout = 2 * time.Second
)

// 从容器的 state.json 中获取信息
func parseContainerState(containerID string) (isSandbox bool, cgroupsPath, sandboxID, sandboxName, sandboxNamespace string, err error) {
	path := filepath.Join(containerStateDir, containerID, "state.json")
	b, err := os.ReadFile(path)
	if err != nil {
		return
	}
	state := gjson.ParseBytes(b)
	if !state.Exists() || !state.IsObject() {
		err = fmt.Errorf("invalid %s", path)
		return
	}
	config := state.Get("config")
	if !config.Exists() || !config.IsObject() {
		fmt.Errorf(".config is not found in %s\n", path)
		return
	}
	cgroupsPathR := config.Get("cgroups").Get("path")
	if !cgroupsPathR.Exists() || cgroupsPathR.String() == "" {
		fmt.Errorf(".config.cgroups.path is not found in %s", path)
		return
	}
	cgroupsPath = cgroupsPathR.String()
	labels := config.Get("labels")
	if !labels.Exists() || !labels.IsArray() {
		fmt.Errorf(".config.labels is not found in %s", path)
		return
	}

	var containerType string
	for _, label := range labels.Array() {
		if !label.Exists() || label.String() == "" {
			continue
		}
		if containerType != "" && sandboxID != "" && sandboxName != "" && sandboxNamespace != "" {
			break
		}

		if strings.HasPrefix(label.String(), labelContainerType+"=") {
			containerType = label.String()[len(labelContainerType)+1:]
			if containerType == typeSandbox {
				isSandbox = true
				return
			}
			continue
		}
		if strings.HasPrefix(label.String(), labelSandboxID+"=") {
			sandboxID = label.String()[len(labelSandboxID)+1:]
			continue
		}
		if strings.HasPrefix(label.String(), labelSandboxName+"=") {
			sandboxName = label.String()[len(labelSandboxName)+1:]
			continue
		}
		if strings.HasPrefix(label.String(), labelSandboxNamespace+"=") {
			sandboxNamespace = label.String()[len(labelSandboxNamespace)+1:]
			continue
		}
	}
	if containerType == "" || sandboxID == "" || sandboxName == "" || sandboxNamespace == "" {
		fmt.Errorf("failed to get some fields from %s", path)
		return
	}
	return
}

// 从 sandbox 的 state.json 中获取信息
func parseSandboxState(sandboxID string) (cgroupsPath string, err error) {
	path := filepath.Join(containerStateDir, sandboxID, "state.json")
	b, err := os.ReadFile(path)
	if err != nil {
		return
	}
	state := gjson.ParseBytes(b)
	if !state.Exists() || !state.IsObject() {
		err = fmt.Errorf("invalid %s", path)
		return
	}
	cgroupsPathR := state.Get("config").Get("cgroups").Get("path")
	if !cgroupsPathR.Exists() || cgroupsPathR.String() == "" {
		err = fmt.Errorf(".config.cgroups.path is not found in %s", path)
		return
	}
	cgroupsPath = cgroupsPathR.String()
	return
}

// 通过 cAdvisor HTTP API /api/v2.0/spec 获取自定义的 labels 信息
func getSandboxLabelPort(cli *CAdvisorClient, cgroupsPath string) (port string, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), requestCAdvisorTimeout)
	defer cancel()
	b, err := cli.requestCAdvisorSpec(ctx, cgroupsPath)
	if err != nil {
		return
	}
	spec := gjson.GetBytes(b, cgroupsPath)
	if !spec.Exists() || !spec.IsObject() {
		err = fmt.Errorf("invalid spec %s", cgroupsPath)
		return
	}
	labels := spec.Get("labels")
	if !labels.Exists() || !labels.IsObject() {
		err = fmt.Errorf(".labels is not found in spec %s", cgroupsPath)
		return
	}
	portR := labels.Get(labelPort)
	if !portR.Exists() || portR.String() == "" {
		err = fmt.Errorf("port is not found in labels of spec %s", cgroupsPath)
		return
	}
	port = portR.String()
	return
}

// 通过 cAdvisor HTTP API /api/v2.0/spec 获取容器的 cpuLimit、memLimit
func getContainerResourceLimit(cli *CAdvisorClient, cgroupsPath string) (cpuLimit, memLimit float64, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), requestCAdvisorTimeout)
	defer cancel()
	b, err := cli.requestCAdvisorSpec(ctx, cgroupsPath)
	if err != nil {
		return
	}
	spec := gjson.GetBytes(b, cgroupsPath)
	if !spec.Exists() || !spec.IsObject() {
		err = fmt.Errorf("invalid spec %s", cgroupsPath)
		return
	}
	cpu := spec.Get("cpu")
	if !cpu.Exists() || !cpu.IsObject() {
		err = fmt.Errorf(".cpu is not found in spec %s", cgroupsPath)
		return
	}
	mem := spec.Get("memory")
	if !mem.Exists() || !mem.IsObject() {
		err = fmt.Errorf(".memory is not found in spec %s", cgroupsPath)
		return
	}
	cpuLimit = cpu.Get("limit").Float()
	memLimit = mem.Get("limit").Float()
	if cpuLimit == 0 || memLimit == 0 {
		err = fmt.Errorf("cpuLimit or memLimit is 0 in spec %s", cgroupsPath)
		return
	}
	return
}

// 需要通过 cAdvisor HTTP API /api/v2.0/summary 获取容器的 cpuUsage、memUsage
func getContainerResourceUsage(cli *CAdvisorClient, cgroupsPath string) (cpuUsage, memUsage float64, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), requestCAdvisorTimeout)
	defer cancel()
	b, err := cli.requestCAdvisorSummary(ctx, cgroupsPath)
	if err != nil {
		return
	}
	summary := gjson.GetBytes(b, cgroupsPath)
	if !summary.Exists() || !summary.IsObject() {
		err = fmt.Errorf("invalid summary %s", cgroupsPath)
		return
	}
	latestUsage := summary.Get("latest_usage")
	if !latestUsage.Exists() || !latestUsage.IsObject() {
		err = fmt.Errorf(".latest_usage is not found in summary %s", cgroupsPath)
		return
	}
	cpuUsage = latestUsage.Get("cpu").Float()
	memUsage = latestUsage.Get("memory").Float()
	return
}
