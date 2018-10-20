package runtime

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sylabs/cri/pkg/namespace"
	k8s "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
)

const (
	podInfoPathFormat = "/var/run/singularity/pods/"
	nsStorePathFormat = "namespaces/"
	resolvConfPath    = "resolv.conf"
	hostnamePath      = "hostname"
)

func podID(meta *k8s.PodSandboxMetadata) string {
	return fmt.Sprintf("%s_%s_%s_%d", meta.GetName(), meta.GetNamespace(), meta.GetUid(), meta.GetAttempt())
}

func ensurePodDirectories(podID string) error {
	err := os.MkdirAll(filepath.Join(podInfoPathFormat, podID, nsStorePathFormat), os.ModePerm)
	if err != nil {
		return fmt.Errorf("could not create directory for pod: %v", err)
	}
	return nil
}

func bindNamespacePath(podID string, nsType specs.LinuxNamespaceType) string {
	return filepath.Join(podInfoPathFormat, podID, nsStorePathFormat, string(nsType))
}

func addResolvConf(podID string, config *k8s.DNSConfig) error {
	resolv, err := os.Create(filepath.Join(podInfoPathFormat, podID, resolvConfPath))
	if err != nil {
		return fmt.Errorf("could not create %s: %v", resolvConfPath, err)
	}

	for _, s := range config.Servers {
		fmt.Fprintf(resolv, "nameserver %s\n", s)
	}
	for _, s := range config.Searches {
		fmt.Fprintf(resolv, "search %s\n", s)
	}
	for _, o := range config.Options {
		fmt.Fprintf(resolv, "options %s\n", o)
	}
	if err = resolv.Close(); err != nil {
		return fmt.Errorf("could not close %s: %v", resolvConfPath, err)
	}

	return nil
}

func addHostname(podID, hostname string) error {
	host, err := os.Create(filepath.Join(podInfoPathFormat, podID, hostnamePath))
	if err != nil {
		return fmt.Errorf("could not create %s: %v", hostnamePath, err)
	}
	fmt.Fprintln(host, hostname)
	if err = host.Close(); err != nil {
		return fmt.Errorf("could not close %s: %v", hostnamePath, err)
	}
	return nil
}

// cleanup is responsible for cleaning any files that were created by pod.
// If noErr is true then any errors occurred during cleanup are ignored.
func cleanupPod(pod *pod, noErr bool) error {
	for _, ns := range pod.namespaces {
		err := namespace.Remove(ns)
		if err != nil && !noErr {
			return fmt.Errorf("could not remove namespace: %v", err)
		}
	}

	err := os.RemoveAll(filepath.Join(podInfoPathFormat, pod.id))
	if err != nil && !noErr {
		return fmt.Errorf("could not cleanup pod: %v", err)
	}

	if pod.config.GetLogDirectory() != "" {
		err := os.RemoveAll(pod.config.GetLogDirectory())
		if err != nil && !noErr {
			return fmt.Errorf("could not remove log directory: %v", err)
		}
	}

	return nil
}

func podMatches(pod *pod, filter *k8s.PodSandboxFilter) bool {
	if filter == nil {
		return true
	}

	if filter.Id != "" && filter.Id != pod.id {
		return false
	}

	if filter.State != nil && filter.State.State != pod.state {
		return false
	}

	for k, v := range filter.LabelSelector {
		lablel, ok := pod.config.Labels[k]
		if !ok {
			return false
		}
		if v != lablel {
			return false
		}
	}
	return true
}
