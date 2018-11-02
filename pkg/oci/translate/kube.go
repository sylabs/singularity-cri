package translate

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"syscall"

	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/runtime-tools/generate"
	"github.com/sylabs/cri/pkg/kube/sandbox"
	"golang.org/x/sys/unix"
	k8s "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
)

type kubeT struct {
	g          generate.Generator
	contConfig *k8s.ContainerConfig
	pod        *sandbox.Pod
}

// KubeToOCI translates container and corresponding pod config into OCI config.
func KubeToOCI(pod *sandbox.Pod, cConf *k8s.ContainerConfig) (*specs.Spec, error) {
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
	t.g.SetRootPath(t.contConfig.GetImage().GetImage())
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

	if t.contConfig.GetLinux().GetSecurityContext().GetPrivileged() {
		mounts := t.g.Mounts()
		for i := range mounts {
			switch mounts[i].Type {
			case "sysfs", "procfs", "proc":
				mounts[i].Options = []string{"nosuid", "noexec", "nodev", "rw"}
			}
		}
	}

	for _, mount := range t.contConfig.GetMounts() {
		source, err := filepath.EvalSymlinks(mount.GetHostPath())
		if err != nil {
			return fmt.Errorf("invalid bind mount source: %v", err)
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
	if t.contConfig.GetLinux().GetSecurityContext().GetPrivileged() {
		err := t.addAllDevices("/dev")
		if err != nil {
			return err
		}
		t.g.Config.Linux.Resources.Devices = []specs.LinuxDeviceCgroup{{Allow: true, Access: "rwm"}}
		return nil
	}

	for _, dev := range t.contConfig.GetDevices() {
		device, err := t.device(dev.GetHostPath(), dev.GetContainerPath())
		if err != nil {
			return err
		}
		t.g.AddDevice(*device)
		t.g.AddLinuxResourcesDevice(true, device.Type, &device.Major, &device.Minor, dev.GetPermissions())
	}
	return nil
}

func (t *kubeT) addAllDevices(dir string) error {
	devices, err := ioutil.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("could not read /dev: %v", err)
	}
	for _, dev := range devices {
		devPath := filepath.Join(dir, dev.Name())
		if dev.IsDir() {
			switch dev.Name() {
			case "pts", "shm", "fd", "mqueue":
				continue
			}
			if err := t.addAllDevices(devPath); err != nil {
				return err
			}
			continue
		}
		device, err := t.device(devPath, devPath)
		if err != nil {
			return err
		}
		t.g.AddDevice(*device)
	}
	return nil
}

func (t *kubeT) device(from, to string) (*specs.LinuxDevice, error) {
	stat, err := os.Stat(from)
	if err != nil {
		return nil, fmt.Errorf("invalid device source: %v", err)
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
	if devType == "" {
		return nil, fmt.Errorf("unsupported device type")
	}
	major := int64(unix.Major(sys.Rdev))
	minor := int64(unix.Minor(sys.Rdev))

	return &specs.LinuxDevice{
		Path:     to,
		Type:     devType,
		Major:    major,
		Minor:    minor,
		FileMode: &mode,
		UID:      &sys.Uid,
		GID:      &sys.Gid,
	}, nil
}

func (t *kubeT) configureNamespaces() {
	if t.pod.GetHostname() != "" {
		t.g.AddOrReplaceLinuxNamespace(specs.UTSNamespace, t.pod.NamespacePath(specs.UTSNamespace))
	}
	t.g.AddOrReplaceLinuxNamespace(specs.MountNamespace, "")

	security := t.contConfig.GetLinux().GetSecurityContext()
	switch security.GetNamespaceOptions().GetIpc() {
	case k8s.NamespaceMode_CONTAINER:
		t.g.AddOrReplaceLinuxNamespace(specs.IPCNamespace, "")
	case k8s.NamespaceMode_POD:
		t.g.AddOrReplaceLinuxNamespace(specs.IPCNamespace, t.pod.NamespacePath(specs.IPCNamespace))
	}
	switch security.GetNamespaceOptions().GetNetwork() {
	case k8s.NamespaceMode_CONTAINER:
		t.g.AddOrReplaceLinuxNamespace(specs.NetworkNamespace, "")
	case k8s.NamespaceMode_POD:
		t.g.AddOrReplaceLinuxNamespace(specs.NetworkNamespace, t.pod.NamespacePath(specs.NetworkNamespace))
	}
	switch security.GetNamespaceOptions().GetPid() {
	case k8s.NamespaceMode_CONTAINER:
		t.g.AddOrReplaceLinuxNamespace(string(specs.PIDNamespace), "")
	case k8s.NamespaceMode_POD:
		t.g.AddOrReplaceLinuxNamespace(string(specs.PIDNamespace), t.pod.NamespacePath(specs.PIDNamespace))
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
	t.g.SetProcessNoNewPrivileges(security.GetNoNewPrivs())
	t.g.SetProcessUsername(security.GetRunAsUsername())
	t.g.SetProcessUID(uint32(security.GetRunAsUser().GetValue()))
	t.g.SetProcessGID(uint32(security.GetRunAsGroup().GetValue()))
	for _, gid := range security.GetSupplementalGroups() {
		t.g.AddProcessAdditionalGid(uint32(gid))
	}
	if security.GetPrivileged() {
		t.g.SetupPrivileged(true)
	} else {
		t.g.SetProcessApparmorProfile(security.GetApparmorProfile())
		for _, capb := range security.GetCapabilities().GetDropCapabilities() {
			t.g.DropProcessCapabilityEffective(capb)
			t.g.DropProcessCapabilityAmbient(capb)
			t.g.DropProcessCapabilityBounding(capb)
			t.g.DropProcessCapabilityInheritable(capb)
			t.g.DropProcessCapabilityPermitted(capb)
		}
		for _, capb := range security.GetCapabilities().GetAddCapabilities() {
			t.g.AddProcessCapabilityEffective(capb)
			t.g.AddProcessCapabilityAmbient(capb)
			t.g.AddProcessCapabilityBounding(capb)
			t.g.AddProcessCapabilityInheritable(capb)
			t.g.AddProcessCapabilityPermitted(capb)
		}
	}
}

func (t *kubeT) configureAnnotations() {
	for k, v := range t.contConfig.GetAnnotations() {
		t.g.AddAnnotation(k, v)
	}
}
