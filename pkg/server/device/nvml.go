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
	"fmt"
	"strings"

	"github.com/NVIDIA/gpu-monitoring-tools/bindings/go/nvml"
	"github.com/golang/glog"
)

func getDevices() ([]*nvml.Device, error) {
	n, err := nvml.GetDeviceCount()
	if err != nil {
		return nil, fmt.Errorf("could not get GPU count: %v", err)
	}

	devices := make([]*nvml.Device, n)
	for i := uint(0); i < n; i++ {
		d, err := nvml.NewDeviceLite(i)
		if err != nil {
			return nil, fmt.Errorf("could not get device #%d: %v", i, err)
		}
		devices[i] = d
	}

	return devices, nil
}

const (
	errGPUMemoryPageFault   = 31
	errGPUStoppedProcessing = 43
	errPreemptiveCleanup    = 45
)

func monitorGPUs(done <-chan struct{}, devIDs []string) (<-chan string, error) {
	ill := make(chan string, len(devIDs))
	eventSet := nvml.NewEventSet()
	for _, devID := range devIDs {
		err := nvml.RegisterEventForDevice(eventSet, nvml.XidCriticalError, devID)
		if err != nil && strings.HasSuffix(err.Error(), "Not Supported") {
			glog.Warningf("Healthcheck is not supported for %s, marking it unhealthy", devID)
			ill <- devID
			continue
		}
		if err != nil {
			nvml.DeleteEventSet(eventSet)
			return nil, fmt.Errorf("could not subscribe for %s events: %v", devID, err)
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
					for _, devID := range devIDs {
						ill <- devID
					}
					continue
				}
				ill <- *event.UUID
			}
		}
	}()
	return ill, nil
}
