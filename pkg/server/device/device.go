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

// Portions of this file were derived from github.com/nvidia/k8s-device-plugin
// under the following license:
//
// Copyright (c) 2017, NVIDIA CORPORATION. All rights reserved.
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions
// are met:
// * Redistributions of source code must retain the above copyright
// notice, this list of conditions and the following disclaimer.
// * Redistributions in binary form must reproduce the above copyright
// notice, this list of conditions and the following disclaimer in the
// documentation and/or other materials provided with the distribution.
// * Neither the name of NVIDIA CORPORATION nor the names of its
// contributors may be used to endorse or promote products derived
// from this software without specific prior written permission.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS ``AS IS'' AND ANY
// EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
// IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR
// PURPOSE ARE DISCLAIMED.  IN NO EVENT SHALL THE COPYRIGHT OWNER OR
// CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL,
// EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO,
// PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR
// PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY
// OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
// (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
// OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

package device

import (
	"context"
	"fmt"

	"github.com/NVIDIA/gpu-monitoring-tools/bindings/go/nvml"
	"github.com/golang/glog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	k8sDP "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1beta1"
)

var (
	// ErrNoGPUs is returned when device plugin is unable to
	// detect any GPU device on the host.
	ErrNoGPUs = fmt.Errorf("GPUs are not found on this host")

	// ErrUnableToLoad is returned when device plugin is unable to
	// detect loaded graphic driver on the host or unable to load
	// NVML shared library.
	ErrUnableToLoad = fmt.Errorf("unable to load: check libnvidia-ml.so.1 library and graphic drivers")
)

// SingularityDevicePlugin is Singularity implementation of a DevicePluginServer
// interface that allows containers to request nvidia GPUs.
type SingularityDevicePlugin struct {
	devices []*k8sDP.Device

	done         chan struct{}
	unhealthyDev <-chan string
}

// NewSingularityDevicePlugin initializes and returns Singularity device plugin
// that allows us to access nvidia GPUs on host. It fails if there is no
// graphic griver installed on host or if Nvidia Management Library (NVML)
// fails to load.
func NewSingularityDevicePlugin() (dp *SingularityDevicePlugin, err error) {
	glog.Infof("Loading NVML")
	if err = nvml.Init(); err != nil {
		glog.Errorf("Could not initialize NVML library: %v", err)
		return nil, ErrUnableToLoad
	}

	dp = &SingularityDevicePlugin{
		done: make(chan struct{}),
	}
	defer func() {
		if err != nil {
			glog.Errorf("Shutting down device plugin due to %v", err)
			dp.Shutdown()
		}
	}()

	v, err := nvml.GetDriverVersion()
	if err != nil {
		glog.Errorf("Could not get driver version: %v", err)
		return nil, ErrUnableToLoad
	}
	glog.Infof("Found graphic driver of version %v", v)

	dp.devices, err = getDevices()
	if err != nil {
		return nil, fmt.Errorf("could not get available devices: %v", err)
	}
	if len(dp.devices) == 0 {
		return nil, ErrNoGPUs
	}

	dp.unhealthyDev, err = monitorGPUs(dp.done, dp.devices)
	if err != nil {
		return nil, fmt.Errorf("could not start GPU monitoring: %v", err)
	}

	return dp, nil
}

// Shutdown shuts down device plugin and any GPU monitoring activity.
func (dp *SingularityDevicePlugin) Shutdown() error {
	glog.Infof("Shutdown of NVML returned: %v", nvml.Shutdown())
	glog.Infof("Cancelling GPU monitoring")
	close(dp.done)
	return nil
}

// GetDevicePluginOptions returns options to be communicated with Device Manager.
func (*SingularityDevicePlugin) GetDevicePluginOptions(context.Context, *k8sDP.Empty) (*k8sDP.DevicePluginOptions, error) {
	return &k8sDP.DevicePluginOptions{
		PreStartRequired: true,
	}, nil
}

// ListAndWatch returns a stream of List of Devices. Whenever a Device state changes
// or a Device disappears, ListAndWatch returns the new list.
func (dp *SingularityDevicePlugin) ListAndWatch(_ *k8sDP.Empty, srv k8sDP.DevicePlugin_ListAndWatchServer) error {
	err := srv.Send(&k8sDP.ListAndWatchResponse{Devices: dp.devices})
	if err != nil {
		return status.Errorf(codes.Unknown, "could not send initial devices state: %v", err)
	}
	for {
		select {
		case <-dp.done:
			return nil
		case devID := <-dp.unhealthyDev:
			for _, dev := range dp.devices {
				if dev.ID == devID {
					dev.Health = k8sDP.Unhealthy
					break
				}
			}
			err := srv.Send(&k8sDP.ListAndWatchResponse{Devices: dp.devices})
			if err != nil {
				return status.Errorf(codes.Unknown, "could not send updated devices state: %v", err)
			}
		}
	}
}

// Allocate is called during container creation so that the Device Plugin can run
// device specific operations and instruct Kubelet of the steps to make the Device
// available in the container.
func (*SingularityDevicePlugin) Allocate(context.Context, *k8sDP.AllocateRequest) (*k8sDP.AllocateResponse, error) {
	glog.Infof("Allocate")
	return &k8sDP.AllocateResponse{}, nil
}

// PreStartContainer is called, if indicated by Device Plugin during registration phase,
// before each container start. Device plugin can run device specific operations
// such as resetting the device before making devices available to the container.
func (*SingularityDevicePlugin) PreStartContainer(context.Context, *k8sDP.PreStartContainerRequest) (*k8sDP.PreStartContainerResponse, error) {
	return &k8sDP.PreStartContainerResponse{}, nil
}
