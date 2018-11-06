// Copyright (c) 2018 Sylabs, Inc. All rights reserved.
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
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/runtime-tools/generate"
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
	t.g.SetRootPath(t.pod.RootfsPath())
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
	t.g.AddOrReplaceLinuxNamespace(string(specs.MountNamespace), "")

	for k, v := range t.pod.GetAnnotations() {
		t.g.AddAnnotation(k, v)
	}
	for k, v := range t.pod.GetLinux().GetSysctls() {
		t.g.AddLinuxSysctl(k, v)
	}

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
