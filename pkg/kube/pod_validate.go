package kube

import (
	"fmt"
	"strings"

	"github.com/opencontainers/runtime-spec/specs-go"
	k8s "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
)

var (
	sysctlToNs = map[string]specs.LinuxNamespaceType{
		"kernel.shm": specs.IPCNamespace,
		"kernel.msg": specs.IPCNamespace,
		"kernel.sem": specs.IPCNamespace,
		"fs.mqueue.": specs.IPCNamespace,
		"net.":       specs.NetworkNamespace,
	}
)

func (p *Pod) validateConfig() error {
	hasIPC := p.GetLinux().GetSecurityContext().GetNamespaceOptions().GetIpc() == k8s.NamespaceMode_POD
	hasNET := p.GetLinux().GetSecurityContext().GetNamespaceOptions().GetNetwork() == k8s.NamespaceMode_POD

	for sysctl := range p.GetLinux().GetSysctls() {
		for prefix, nsType := range sysctlToNs {
			if strings.HasPrefix(sysctl, prefix) {
				if (nsType == specs.IPCNamespace && !hasIPC) ||
					(nsType == specs.NetworkNamespace && !hasNET) {
					return fmt.Errorf("sysctl %s requires a separate %s namespace", sysctl, nsType)
				}
			}
		}
	}

	return nil
}
