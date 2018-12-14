package kube

import (
	"fmt"
	"log"
	"strings"
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
		log.Printf("setting AppArmor profile to %q", aaProfile)
		security.ApparmorProfile = aaProfile
	}
	if security != nil {
		scProfile, err := prepareSeccompPath(security.GetSeccompProfilePath())
		if err != nil {
			return fmt.Errorf("invalid Seccomp profile path: %v", err)
		}
		security.SeccompProfilePath = scProfile
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
	log.Printf("setting Seccomp profile to %q", scProfile)
	return scProfile, nil
}
