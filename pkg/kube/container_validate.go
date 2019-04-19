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
	"strings"

	"github.com/sylabs/singularity/pkg/util/capabilities"
	"k8s.io/klog"
)

const (
	appArmorLocalhostPrefix = "localhost/"
	seccompLocalhostPrefix  = "localhost/"

	defaultAppArmorProfile      = "runtime/default"
	defaultSeccompProfile       = "runtime/default"
	defaultDockerSeccompProfile = "docker/default"
	unconfinedSeccompProfile    = "unconfined"
)

func (c *Container) validateConfig() error {
	security := c.GetLinux().GetSecurityContext()
	aaProfile := security.GetApparmorProfile()
	selinuxOptions := security.GetSelinuxOptions()

	if aaProfile != "" && selinuxOptions != nil {
		return fmt.Errorf("cannot use both AppArmour profile and SELinux options")
	}

	if aaProfile != "" {
		if aaProfile == defaultAppArmorProfile {
			aaProfile = "" // do not specify anything in that case
		}
		aaProfile = strings.TrimPrefix(aaProfile, appArmorLocalhostPrefix)
		klog.Infof("Setting AppArmor profile to %q", aaProfile)
		security.ApparmorProfile = aaProfile
	}
	if security != nil {
		scProfile, err := prepareSeccompPath(security.GetSeccompProfilePath())
		if err != nil {
			return fmt.Errorf("invalid Seccomp profile path: %v", err)
		}
		security.SeccompProfilePath = scProfile
	}
	caps := security.GetCapabilities()
	if caps != nil {
		caps.AddCapabilities = prepareCapabilities(caps.AddCapabilities, nil)
		caps.DropCapabilities = prepareCapabilities(caps.DropCapabilities, caps.AddCapabilities)
	}
	return nil
}

func prepareSeccompPath(scProfile string) (string, error) {
	if scProfile == "" || scProfile == unconfinedSeccompProfile {
		// empty profile equals to unconfined according to docs
		return unconfinedSeccompProfile, nil
	}
	if scProfile == defaultSeccompProfile || scProfile == defaultDockerSeccompProfile {
		// set runtime default profile - nothing in our case
		return "", nil
	}
	if !strings.HasPrefix(scProfile, seccompLocalhostPrefix) {
		return "", fmt.Errorf("custom profiles without %q prefix are not allowed", seccompLocalhostPrefix)
	}
	scProfile = strings.TrimPrefix(scProfile, seccompLocalhostPrefix)
	klog.Infof("Setting Seccomp profile to %q", scProfile)
	return scProfile, nil
}

func prepareCapabilities(caps []string, excluded []string) []string {
	normalized, unknown := capabilities.Normalize(caps)
	if len(unknown) != 0 {
		klog.Warningf("Skipping unknown capabilities: %v", unknown)
	}
	// remove excluded capabilities if any from normalized set
	for i := len(normalized) - 1; i >= 0; i-- {
		for _, exclude := range excluded {
			if exclude == normalized[i] {
				normalized = append(normalized[:i], normalized[i+1:]...)
			}
		}
	}
	return normalized
}
