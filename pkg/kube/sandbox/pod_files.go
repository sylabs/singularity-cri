package sandbox

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sylabs/cri/pkg/namespace"
)

const (
	podInfoPath = "/var/run/singularity/pods/"

	nsStorePath    = "namespaces/"
	resolvConfPath = "resolv.conf"
	hostnamePath   = "hostname"

	bundleStorePath = "bundle/"
	rootfsStorePath = "rootfs/"
	ociConfigPath   = "config.json"
)

// PathToNamespace returns path to pod's namespace file of the passed type.
// If requested namespace is not unshared specifically for pod an empty
// string is returned.
func (p *Pod) PathToNamespace(nsType specs.LinuxNamespaceType) string {
	for _, ns := range p.namespaces {
		if ns.Type == nsType {
			return p.bindNamespacePath(nsType)
		}
	}
	return ""
}

// HostnameFilePath returns path to pod's hostname file.
func (p *Pod) HostnameFilePath() string {
	return filepath.Join(podInfoPath, p.id, hostnamePath)
}

// ResolvConfFilePath returns path to pod's resolv.conf file.
func (p *Pod) ResolvConfFilePath() string {
	return filepath.Join(podInfoPath, p.id, resolvConfPath)
}

// bundlePath returns path to pod's filesystem bundle directory.
func (p *Pod) bundlePath() string {
	return filepath.Join(podInfoPath, p.id, bundleStorePath)
}

// rootfsPath returns path to pod's rootfs directory.
func (p *Pod) rootfsPath() string {
	return filepath.Join(podInfoPath, p.id, bundleStorePath, rootfsStorePath)
}

// ociConfigPath returns path to pod's config.json file.
func (p *Pod) ociConfigPath() string {
	return filepath.Join(podInfoPath, p.id, bundleStorePath, ociConfigPath)
}

// bindNamespacePath returns path to pod's namespace file of the passed type.
func (p *Pod) bindNamespacePath(nsType specs.LinuxNamespaceType) string {
	return filepath.Join(podInfoPath, p.id, nsStorePath, string(nsType))
}

func (p *Pod) prepareFiles() error {
	err := os.MkdirAll(filepath.Join(podInfoPath, p.id, nsStorePath), os.ModePerm)
	if err != nil {
		return fmt.Errorf("could not create directory for pod: %v", err)
	}
	err = os.MkdirAll(filepath.Join(podInfoPath, p.id, bundleStorePath, rootfsStorePath), os.ModePerm)
	if err != nil {
		return fmt.Errorf("could not create rootfs directory for pod: %v", err)
	}

	if err = p.addLogDirectory(); err != nil {
		return fmt.Errorf("could not create log directory: %v", err)
	}
	if err = p.addResolvConf(); err != nil {
		return fmt.Errorf("could not create resolv.conf: %v", err)
	}
	if err = p.addHostname(); err != nil {
		return fmt.Errorf("could not create hostname file: %v", err)
	}
	if err = p.addOCIConfig(); err != nil {
		return fmt.Errorf("could not create config.json: %v", err)
	}
	return nil
}

func (p *Pod) addResolvConf() error {
	config := p.GetDnsConfig()
	if config == nil {
		return nil
	}

	resolv, err := os.OpenFile(p.ResolvConfFilePath(), os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("could not create %s: %v", resolvConfPath, err)
	}
	for _, s := range config.GetServers() {
		fmt.Fprintf(resolv, "nameserver %s\n", s)
	}
	for _, s := range config.GetSearches() {
		fmt.Fprintf(resolv, "search %s\n", s)
	}
	for _, o := range config.GetOptions() {
		fmt.Fprintf(resolv, "options %s\n", o)
	}
	if err = resolv.Close(); err != nil {
		return fmt.Errorf("could not close %s: %v", resolvConfPath, err)
	}
	return nil
}

func (p *Pod) addHostname() error {
	hostname := p.GetHostname()
	if hostname == "" {
		return nil
	}

	host, err := os.OpenFile(p.HostnameFilePath(), os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("could not create %s: %v", hostnamePath, err)
	}
	fmt.Fprintln(host, hostname)
	if err = host.Close(); err != nil {
		return fmt.Errorf("could not close %s: %v", hostnamePath, err)
	}
	return nil
}

func (p *Pod) addLogDirectory() error {
	logDir := p.GetLogDirectory()
	if logDir == "" {
		return nil
	}

	err := os.MkdirAll(logDir, os.ModePerm)
	if err != nil {
		return fmt.Errorf("could not create %s: %v", logDir, err)
	}
	return nil
}

func (p *Pod) addOCIConfig() error {
	config, err := os.OpenFile(p.ociConfigPath(), os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("could not create OCI config file: %v", err)
	}
	if err = config.Close(); err != nil {
		return fmt.Errorf("could not close %s: %v", ociConfigPath, err)
	}
	return nil
}

// cleanup is responsible for cleaning any files that were created by pod.
// If silent is true then any errors occurred during cleanup are ignored.
func (p *Pod) cleanup(silent bool) error {
	for _, ns := range p.namespaces {
		err := namespace.Remove(ns)
		if err != nil && !silent {
			return fmt.Errorf("could not remove namespace: %v", err)
		}
	}
	err := os.RemoveAll(filepath.Join(podInfoPath, p.id))
	if err != nil && !silent {
		return fmt.Errorf("could not cleanup pod: %v", err)
	}
	if p.GetLogDirectory() != "" {
		err := os.RemoveAll(p.GetLogDirectory())
		if err != nil && !silent {
			return fmt.Errorf("could not remove log directory: %v", err)
		}
	}
	return nil
}
