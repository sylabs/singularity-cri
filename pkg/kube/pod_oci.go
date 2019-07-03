// Copyright (c) 2018-2019 Sylabs, Inc. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package kube

import (
	"fmt"
	"io/ioutil"

	"github.com/golang/glog"
	"github.com/kubernetes-sigs/cri-o/pkg/seccomp"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/runtime-tools/generate"
	"github.com/opencontainers/selinux/go-selinux/label"
	k8s "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
)

type podTranslator struct {
	pod *Pod
	g   generate.Generator
}

// translatePod translates Pod instance into OCI container specification.
func translatePod(pod *Pod) (*specs.Spec, error) {
	g := generate.Generator{
		Config: &specs.Spec{
			Version: specs.Version,
		},
	}
	t := podTranslator{
		g:   g,
		pod: pod,
	}
	return t.translate()
}

func (t *podTranslator) translate() (*specs.Spec, error) {
	t.g.SetRootPath(t.pod.rootfsPath())
	t.g.SetRootReadonly(false)

	t.g.SetHostname(t.pod.GetHostname())
	t.g.AddMount(specs.Mount{
		Destination: "/proc",
		Source:      "proc",
		Type:        "proc",
	})
	t.g.AddMount(specs.Mount{
		Destination: "/dev",
		Type:        "tmpfs",
		Source:      "tmpfs",
		Options:     []string{"nosuid", "strictatime", "mode=755", "size=65536k"},
	})
	t.g.SetProcessCwd("/")
	t.g.SetProcessArgs([]string{"true"})

	for _, ns := range t.pod.namespaces {
		t.g.AddOrReplaceLinuxNamespace(string(ns.Type), ns.Path)
	}
	t.g.AddOrReplaceLinuxNamespace(string(specs.MountNamespace), "")

	for k, v := range t.pod.GetAnnotations() {
		t.g.AddAnnotation(k, v)
	}
	for k, v := range t.pod.GetLinux().GetSysctls() {
		t.g.AddLinuxSysctl(k, v)
	}

	security := t.pod.GetLinux().GetSecurityContext()
	if err := setupSELinux(t, security.GetSelinuxOptions()); err != nil {
		return nil, err
	}
	if err := setupSeccomp(&t.g, security.GetSeccompProfilePath()); err != nil {
		return nil, err
	}

	t.g.SetLinuxCgroupsPath(t.pod.GetLinux().GetCgroupParent())
	t.g.SetRootReadonly(security.GetReadonlyRootfs())
	t.g.SetProcessUID(uint32(security.GetRunAsUser().GetValue()))
	t.g.SetProcessGID(uint32(security.GetRunAsGroup().GetValue()))
	for _, gid := range security.GetSupplementalGroups() {
		t.g.AddProcessAdditionalGid(uint32(gid))
	}

	// simply apply privileged at the end of the config
	t.g.SetupPrivileged(security.GetPrivileged())
	return t.g.Config, nil
}

func setupSELinux(t *podTranslator, options *k8s.SELinuxOption) error {
	if options == nil {
		return nil
	}

	var labels []string
	if options.GetUser() != "" {
		labels = append(labels, "user:"+options.GetUser())
	}
	if options.GetRole() != "" {
		labels = append(labels, "role:"+options.GetRole())
	}
	if options.GetType() != "" {
		labels = append(labels, "type:"+options.GetType())
	}
	if options.GetLevel() != "" {
		labels = append(labels, "level:"+options.GetLevel())
	}
	processLabel, mountLabel, err := label.InitLabels(labels)
	if err != nil {
		return fmt.Errorf("could not init selinux labels: %v", err)
	}
	glog.V(3).Infof("Setting mount label to %q for pod %s", mountLabel, t.pod.id)
	t.g.SetLinuxMountLabel(mountLabel)
	glog.V(3).Infof("Setting process's SELinux label to %q for pod %s", processLabel, t.pod.id)
	t.g.SetProcessSelinuxLabel(processLabel)
	return nil
}

func setupSeccomp(g *generate.Generator, profile string) error {
	if profile == "" {
		return nil
	}
	if g.Config.Linux == nil {
		g.Config.Linux = new(specs.Linux)
	}
	if profile == unconfinedSeccompProfile {
		// drop any default config
		g.Config.Linux.Seccomp = nil
		return nil
	}

	data, err := ioutil.ReadFile(profile)
	if err != nil {
		return fmt.Errorf("could not read seccomp profile: %v", err)
	}
	if g.Config.Process == nil {
		g.Config.Process = new(specs.Process)
	}
	if g.Config.Process.Capabilities == nil {
		g.Config.Process.Capabilities = new(specs.LinuxCapabilities)
	}
	if err := seccomp.LoadProfileFromBytes(data, g); err != nil {
		return fmt.Errorf("could not setup seccomp: %v", err)
	}
	return nil
}
