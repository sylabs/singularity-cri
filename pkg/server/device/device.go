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

package device

import (
	"context"

	"github.com/golang/glog"
	k8s "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1beta1"
)

// SingularityDevicePlugin is Singularity implementation of a DevicePluginServer
// interface that allows containers to request nvidia GPUs.
type SingularityDevicePlugin struct {
}

func NewSingularityDevicePlugin() (*SingularityDevicePlugin, error) {
	return &SingularityDevicePlugin{}, nil
}

func (dp *SingularityDevicePlugin) Shutdown() error {
	return nil
}

// GetDevicePluginOptions returns options to be communicated with Device Manager.
func (*SingularityDevicePlugin) GetDevicePluginOptions(context.Context, *k8s.Empty) (*k8s.DevicePluginOptions, error) {
	glog.Infof("GetDevicePluginOptions")
	return &k8s.DevicePluginOptions{}, nil
}

// ListAndWatch returns a stream of List of Devices. Whenever a Device state changes
// or a Device disappears, ListAndWatch returns the new list.
func (*SingularityDevicePlugin) ListAndWatch(*k8s.Empty, k8s.DevicePlugin_ListAndWatchServer) error {
	glog.Infof("ListAndWatch")
	return nil
}

// Allocate is called during container creation so that the Device Plugin can run
// device specific operations and instruct Kubelet of the steps to make the Device
// available in the container.
func (*SingularityDevicePlugin) Allocate(context.Context, *k8s.AllocateRequest) (*k8s.AllocateResponse, error) {
	glog.Infof("Allocate")
	return &k8s.AllocateResponse{}, nil
}

// PreStartContainer is called, if indicated by Device Plugin during registration phase,
// before each container start. Device plugin can run device specific operations
// such as resetting the device before making devices available to the container.
func (*SingularityDevicePlugin) PreStartContainer(context.Context, *k8s.PreStartContainerRequest) (*k8s.PreStartContainerResponse, error) {
	glog.Infof("PreStartContainer")
	return &k8s.PreStartContainerResponse{}, nil
}
