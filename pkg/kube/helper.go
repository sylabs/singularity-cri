package kube

import (
	"fmt"
	"os"
	"strings"

	"github.com/golang/glog"
	k8s "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
)

func writeResolvConf(path string, config *k8s.DNSConfig) error {
	if config == nil {
		return nil
	}

	glog.V(8).Infof("Creating resolv.conf file %s", path)
	resolv, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("could not create %s: %v", podResolvConfPath, err)
	}
	for _, s := range config.GetServers() {
		fmt.Fprintf(resolv, "nameserver %s\n", s)
	}
	if len(config.GetSearches()) > 0 {
		fmt.Fprintf(resolv, "search %s\n", strings.Join(config.GetSearches(), ", "))
	}
	for _, o := range config.GetOptions() {
		fmt.Fprintf(resolv, "options %s\n", o)
	}
	if err = resolv.Close(); err != nil {
		return fmt.Errorf("could not close %s: %v", podResolvConfPath, err)
	}
	return nil
}
