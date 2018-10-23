package translate

import (
	"fmt"
	"os"
	"syscall"

	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/runtime-tools/generate"
	"github.com/sylabs/cri/pkg/kube"
	"golang.org/x/sys/unix"
	k8s "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
)

type kubeT struct {
	g          generate.Generator
	contConfig *k8s.ContainerConfig
	pod        *kube.Pod
}

// KubeToOCI translates container and corresponding pod config into OCI config.
func KubeToOCI(cConf *k8s.ContainerConfig, pod *kube.Pod) (*specs.Spec, error) {
	g, err := generate.New("linux")
	if err != nil {
		return nil, fmt.Errorf("could not initialize generator: %v", err)
	}
	t := kubeT{
		g:          g,
		contConfig: cConf,
		pod:        pod,
	}
	return t.translate()
}

func (t *kubeT) translate() (*specs.Spec, error) {
	t.configureImage()
	if err := t.configureDevices(); err != nil {
		return nil, fmt.Errorf("could not configure devices: %v", err)
	}
	if err := t.configureMounts(); err != nil {
		return nil, fmt.Errorf("could not configure mounts: %v", err)
	}
	t.configureNamespaces()
	t.configureResources()
	t.configureProcess()
	t.configureAnnotations()
	return t.g.Config, nil
}

func (t *kubeT) configureImage() {
	image := t.contConfig.GetImage()
	t.g.SetRootPath(image.Image)
	t.g.SetRootReadonly(t.contConfig.GetLinux().GetSecurityContext().GetReadonlyRootfs())
}

func (t *kubeT) configureMounts() error {
	if t.pod.GetDnsConfig() != nil {
		t.g.AddMount(specs.Mount{
			Destination: "/etc/resolv.conf",
			Source:      t.pod.ResolvConfFilePath(),
			Options:     []string{"bind", "ro"},
		})
	}
	if t.pod.GetHostname() != "" {
		t.g.SetHostname(t.pod.GetHostname())
		t.g.AddMount(specs.Mount{
			Destination: "/etc/hostname",
			Source:      t.pod.HostnameFilePath(),
			Options:     []string{"bind", "ro"},
		})
	}

	for _, mount := range t.contConfig.GetMounts() {
		source := mount.GetHostPath()
		mi, err := os.Lstat(source)
		if err != nil {
			return fmt.Errorf("invalid bind mount source: %v", err)
		}
		if mi.Mode()&os.ModeSymlink == os.ModeSymlink {
			source, err = os.Readlink(source)
			if err != nil {
				return fmt.Errorf("could follow mount source symlink: %v", err)
			}
		}

		volume := specs.Mount{
			Source:      source,
			Destination: mount.GetContainerPath(),
			Options:     []string{"rbind"},
		}
		if mount.GetReadonly() {
			volume.Options = append(volume.Options, "ro")
		}
		switch mount.GetPropagation() {
		case k8s.MountPropagation_PROPAGATION_PRIVATE:
			volume.Options = append(volume.Options, "rprivate")
		case k8s.MountPropagation_PROPAGATION_HOST_TO_CONTAINER:
			volume.Options = append(volume.Options, "rslave")
		case k8s.MountPropagation_PROPAGATION_BIDIRECTIONAL:
			volume.Options = append(volume.Options, "rshared")
		}
		t.g.AddMount(volume)
	}
	return nil
}

func (t *kubeT) configureDevices() error {
	for _, dev := range t.contConfig.GetDevices() {
		stat, err := os.Stat(dev.GetHostPath())
		if err != nil {
			return fmt.Errorf("invalid device source: %v", err)
		}
		sys := stat.Sys().(*syscall.Stat_t)

		mode := stat.Mode()
		var devType string
		if mode&syscall.S_IFBLK == syscall.S_IFBLK {
			devType = "b"
		}
		if mode&syscall.S_IFCHR == syscall.S_IFCHR {
			devType = "c"
		}
		if mode&syscall.S_IFIFO == syscall.S_IFIFO {
			devType = "p"
		}
		if mode&syscall.S_IFSOCK == syscall.S_IFSOCK {
			devType = "u"
		}
		major := int64(unix.Major(sys.Rdev))
		minor := int64(unix.Minor(sys.Rdev))

		device := specs.LinuxDevice{
			Path:     dev.GetContainerPath(),
			Type:     devType,
			Major:    major,
			Minor:    minor,
			FileMode: &mode,
			UID:      &sys.Uid,
			GID:      &sys.Gid,
		}
		t.g.AddDevice(device)
		t.g.AddLinuxResourcesDevice(true, devType, &major, &minor, dev.GetPermissions())
	}
	return nil
}

func (t *kubeT) configureNamespaces() {

	if t.pod.GetHostname() != "" {
		t.g.AddOrReplaceLinuxNamespace(specs.UTSNamespace, t.pod.BindNamespacePath(specs.UTSNamespace))
	}
	t.g.AddOrReplaceLinuxNamespace(specs.MountNamespace, "")
	t.g.AddOrReplaceLinuxNamespace(string(specs.PIDNamespace), "")

	security := t.contConfig.GetLinux().GetSecurityContext()
	switch security.GetNamespaceOptions().GetIpc() {
	case k8s.NamespaceMode_CONTAINER:
		t.g.AddOrReplaceLinuxNamespace(specs.IPCNamespace, "")
	case k8s.NamespaceMode_POD:
		t.g.AddOrReplaceLinuxNamespace(specs.IPCNamespace, t.pod.BindNamespacePath(specs.IPCNamespace))
	}
	switch security.GetNamespaceOptions().GetNetwork() {
	case k8s.NamespaceMode_CONTAINER:
		t.g.AddOrReplaceLinuxNamespace(specs.NetworkNamespace, "")
	case k8s.NamespaceMode_POD:
		t.g.AddOrReplaceLinuxNamespace(specs.NetworkNamespace, t.pod.BindNamespacePath(specs.NetworkNamespace))
	}
}

func (t *kubeT) configureResources() {
	res := t.contConfig.GetLinux().GetResources()
	t.g.SetLinuxResourcesCPUPeriod(uint64(res.GetCpuPeriod()))
	t.g.SetLinuxResourcesCPUQuota(res.GetCpuQuota())
	t.g.SetLinuxResourcesCPUMems(res.GetCpusetMems())
	t.g.SetLinuxResourcesCPUCpus(res.GetCpusetCpus())
	t.g.SetLinuxResourcesCPUShares(uint64(res.GetCpuShares()))
	t.g.SetProcessOOMScoreAdj(int(res.GetOomScoreAdj()))
	t.g.SetLinuxResourcesMemoryLimit(res.GetMemoryLimitInBytes())
}

func (t *kubeT) configureProcess() {
	for _, env := range t.contConfig.GetEnvs() {
		t.g.AddProcessEnv(env.GetKey(), env.GetValue())
	}
	t.g.SetProcessCwd(t.contConfig.GetWorkingDir())
	t.g.SetProcessArgs(append(t.contConfig.GetCommand(), t.contConfig.GetArgs()...))
	t.g.SetProcessTerminal(t.contConfig.GetTty())

	security := t.contConfig.GetLinux().GetSecurityContext()
	t.g.SetProcessApparmorProfile(security.GetApparmorProfile())
	t.g.SetProcessNoNewPrivileges(security.GetNoNewPrivs())
	t.g.SetProcessUsername(security.GetRunAsUsername())
	t.g.SetProcessUID(uint32(security.GetRunAsUser().GetValue()))
	t.g.SetProcessGID(uint32(security.GetRunAsGroup().GetValue()))
	for _, gid := range security.GetSupplementalGroups() {
		t.g.AddProcessAdditionalGid(uint32(gid))
	}
	for _, capb := range security.GetCapabilities().GetDropCapabilities() {
		t.g.DropProcessCapabilityEffective(capb)
	}
	for _, capb := range security.GetCapabilities().GetAddCapabilities() {
		t.g.AddProcessCapabilityEffective(capb)
	}
}

func (t *kubeT) configureAnnotations() {
	for k, v := range t.contConfig.GetAnnotations() {
		t.g.AddAnnotation(k, v)
	}
}
