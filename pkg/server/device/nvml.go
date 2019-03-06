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
	"strings"

	"github.com/NVIDIA/gpu-monitoring-tools/bindings/go/nvml"
	"github.com/golang/glog"
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

const (
	errGPUMemoryPageFault   = 31
	errGPUStoppedProcessing = 43
	errPreemptiveCleanup    = 45
)

func monitorGPUs(done <-chan struct{}, devices []*k8s.Device) (<-chan string, error) {
	ill := make(chan string, len(devices))
	eventSet := nvml.NewEventSet()
	for _, dev := range devices {
		err := nvml.RegisterEventForDevice(eventSet, nvml.XidCriticalError, dev.ID)
		if err != nil && strings.HasSuffix(err.Error(), "Not Supported") {
			glog.Warningf("Healthcheck is not supported for %s, marking it unhealthy", dev)
			ill <- dev.ID
			continue
		}
		if err != nil {
			nvml.DeleteEventSet(eventSet)
			return nil, fmt.Errorf("could not subscribe for %s events: %v", dev, err)
		}
	}

	go func() {
		defer nvml.DeleteEventSet(eventSet)

		for {
			select {
			case <-done:
				return
			default:
				event, err := nvml.WaitForEvent(eventSet, 5000)
				if err != nil && strings.Contains(err.Error(), "Timeout") {
					continue
				}
				if err != nil {
					glog.Errorf("Could not wait for event: %v", err)
					continue
				}

				// Application errors: the GPU should still be healthy
				// http://docs.nvidia.com/deploy/xid-errors/index.html#topic_4
				if event.Edata == errGPUMemoryPageFault ||
					event.Edata == errGPUStoppedProcessing ||
					event.Edata == errPreemptiveCleanup {
					continue
				}

				if event.UUID == nil || len(*event.UUID) == 0 {
					// All devices are unhealthy
					for _, dev := range devices {
						ill <- dev.ID
					}
					continue
				}
				ill <- *event.UUID
			}
		}
	}()
	return ill, nil
}
