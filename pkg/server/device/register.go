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
	"time"

	"github.com/golang/glog"
	"google.golang.org/grpc"
	k8sDP "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1beta1"
)

const resourceName = "nvidia.com/gpu"

// RegisterInKubelet registers Singularity device plugin that is
// listening on socket in kubelet.
func RegisterInKubelet(socket string) error {
	for attempt := 1; attempt < 5; attempt++ {
		err := register(socket)
		if err != nil {
			glog.Errorf("Registration failed: %v", err)
			timeout := time.Second * time.Duration(attempt*2)
			glog.Errorf("Retrying in %f seconds", timeout.Seconds())
			time.Sleep(timeout)
			continue
		}
		return nil
	}
	return fmt.Errorf("failed to register in kubelet")
}

func register(socket string) error {
	conn, err := grpc.Dial("unix://"+k8sDP.KubeletSocket, grpc.WithInsecure())
	if err != nil {
		return fmt.Errorf("could not dial kubelet: %v", err)
	}
	defer conn.Close()

	client := k8sDP.NewRegistrationClient(conn)
	req := &k8sDP.RegisterRequest{
		Version:      k8sDP.Version,
		Endpoint:     socket,
		ResourceName: resourceName,
	}

	_, err = client.Register(context.Background(), req)
	if err != nil {
		return fmt.Errorf("could not register in kubelet: %v", err)
	}
	return nil
}
