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
	"fmt"

	"github.com/NVIDIA/gpu-monitoring-tools/bindings/go/nvml"
	k8s "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1beta1"
)

func getDevices() ([]*k8s.Device, error) {
	n, err := nvml.GetDeviceCount()
	if err != nil {
		return nil, fmt.Errorf("could not get GPU count: %v", err)
	}

	devices := make([]*k8s.Device, n)
	for i := uint(0); i < n; i++ {
		d, err := nvml.NewDeviceLite(i)
		if err != nil {
			return nil, fmt.Errorf("could not get device #%d: %v", i, err)
		}
		devices[i] = &k8s.Device{
			ID:     d.UUID,
			Health: k8s.Healthy,
		}
	}

	return devices, nil
}
