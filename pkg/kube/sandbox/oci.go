package sandbox

import (
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/runtime-tools/generate"
)

type ociTranslator struct {
	pod *Pod
	g   generate.Generator
}

func generateOCI(pod *Pod) (*specs.Spec, error) {
	g := generate.Generator{
		Config: &specs.Spec{
			Version: specs.Version,
		},
	}
	t := ociTranslator{
		g:   g,
		pod: pod,
	}
	return t.translate()
}

func (t *ociTranslator) translate() (*specs.Spec, error) {
	t.g.SetRootPath(t.pod.rootfsPath())
	t.g.SetRootReadonly(false)

	t.g.SetHostname(t.pod.GetHostname())
	t.g.AddMount(specs.Mount{
		Destination: "/proc",
		Source:      "proc",
		Type:        "proc",
	})
	t.g.SetProcessCwd("/")
	t.g.SetProcessArgs([]string{"true"})

	for _, ns := range t.pod.namespaces {
		t.g.AddOrReplaceLinuxNamespace(string(ns.Type), ns.Path)
	}

	for k, v := range t.pod.GetAnnotations() {
		t.g.AddAnnotation(k, v)
	}
	for k, v := range t.pod.GetLinux().GetSysctls() {
		t.g.AddLinuxSysctl(k, v)
	}

	// todo add hook
	t.g.AddPostStartHook(specs.Hook{
		Path:    "",
		Args:    nil,
		Env:     nil,
		Timeout: nil,
	})

	security := t.pod.GetLinux().GetSecurityContext()
	t.g.SetupPrivileged(security.GetPrivileged())
	t.g.SetRootReadonly(security.GetReadonlyRootfs())
	t.g.SetProcessUID(uint32(security.GetRunAsUser().GetValue()))
	t.g.SetProcessGID(uint32(security.GetRunAsGroup().GetValue()))
	for _, gid := range security.GetSupplementalGroups() {
		t.g.AddProcessAdditionalGid(uint32(gid))
	}

	return t.g.Config, nil
}
