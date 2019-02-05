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
	"os"
	"strconv"

	"github.com/containerd/cgroups"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sylabs/singularity-cri/pkg/fs"
	k8s "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
)

// ContainerStat holds information about container resources usage.
type ContainerStat struct {
	// Writable layer fs usage.
	Fs *fs.UsageInfo
	// Total memory used by container in bytes
	Memory uint64
	// Total CPU used in nanoseconds.
	CPU uint64
}

// Stat fetches information about container resources usage. This method
// implies that cpuacct and memory cgroups controllers are mounted on host
// at /sys/fs/cgroups/cpuacct and  /sys/fs/cgroups/memory respectively.
func (c *Container) Stat() (*ContainerStat, error) {
	fsInfo, err := fs.Usage(c.baseDir)
	if err != nil {
		return nil, fmt.Errorf("could not get fs usage: %v", err)
	}
	cgroup, err := cgroups.Load(cgroups.V1, cgroups.PidPath(c.Pid()))
	if err != nil {
		return nil, fmt.Errorf("could not load cgroups: %v", err)
	}

	metrics, err := cgroup.Stat(cgroups.IgnoreNotExist)
	if err != nil {
		return nil, fmt.Errorf("could not fetch metrics: %v", err)
	}

	var cpuTotal uint64
	var memoryTotal uint64
	if metrics.CPU != nil && metrics.CPU.Usage != nil {
		cpuTotal = metrics.CPU.Usage.Total
	}
	if metrics.Memory != nil && metrics.Memory.Usage != nil {
		memoryTotal = metrics.Memory.Usage.Usage
	}

	return &ContainerStat{
		Fs:     fsInfo,
		Memory: memoryTotal,
		CPU:    cpuTotal,
	}, nil
}

// UpdateResources updates container resources according to the passed request.
// This method implies that cpu, cpuset and memory cgroups controllers are mounted on host
// at /sys/fs/cgroups/cpu, /sys/fs/cgroups/cpuset  and  /sys/fs/cgroups/memory respectively.
func (c *Container) UpdateResources(upd *k8s.LinuxContainerResources) error {
	var (
		cpuPeriod   *uint64
		cpuQuota    *int64
		cpuShares   *uint64
		memoryLimit *int64
	)
	if upd.MemoryLimitInBytes != 0 {
		memoryLimit = &upd.MemoryLimitInBytes
	}
	if upd.GetCpuPeriod() != 0 {
		cpuPeriod = new(uint64)
		*cpuPeriod = uint64(upd.GetCpuPeriod())
	}
	if upd.GetCpuQuota() != 0 {
		cpuQuota = new(int64)
		*cpuQuota = upd.GetCpuQuota()
	}
	if upd.GetCpuShares() != 0 {
		cpuShares = new(uint64)
		*cpuShares = uint64(upd.GetCpuShares())
	}
	req := &specs.LinuxResources{
		Devices: nil,
		Memory: &specs.LinuxMemory{
			Limit: memoryLimit,
		},
		CPU: &specs.LinuxCPU{
			Shares: cpuShares,
			Quota:  cpuQuota,
			Period: cpuPeriod,
			Cpus:   upd.CpusetCpus,
			Mems:   upd.CpusetMems,
		},
	}
	err := c.cli.UpdateContainerResources(c.ID(), req)
	if err != nil {
		return fmt.Errorf("could not update resources: %v", err)
	}

	if upd.OomScoreAdj != 0 {
		oomAdj, err := os.OpenFile(fmt.Sprintf("/proc/%d/oom_adj", c.Pid()), os.O_WRONLY, 0644)
		if err != nil {
			return fmt.Errorf("could not open oom_adj for container: %v", err)
		}
		defer oomAdj.Close()

		_, err = oomAdj.WriteString(strconv.FormatInt(upd.OomScoreAdj, 32))
		if err != nil {
			return fmt.Errorf("could not update oom_adj for container: %v", err)
		}
	}
	return nil
}
