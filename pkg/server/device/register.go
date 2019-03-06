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
	"fmt"

	"google.golang.org/grpc"
	k8s "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1beta1"
)

const resourceName = "nvidia.com/gpu"

// RegisterInKubelet registers Singularity device plugin that is
// listening on socket in kubelet.
func RegisterInKubelet(socket string) error {
	conn, err := grpc.Dial(k8s.KubeletSocket, grpc.WithInsecure())
	if err != nil {
		return fmt.Errorf("could not dial kubelet: %v", err)
	}
	defer conn.Close()

	client := k8s.NewRegistrationClient(conn)
	req := &k8s.RegisterRequest{
		Version:      k8s.Version,
		Endpoint:     socket,
		ResourceName: resourceName,
	}

	_, err = client.Register(context.Background(), req)
	if err != nil {
		return fmt.Errorf("could not register in kubelet: %v", err)
	}
	return nil
}
