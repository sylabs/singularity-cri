// Copyright (c) 2018 Sylabs, Inc. All rights reserved.
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

package runtime

import (
	"fmt"
	"log"
	"syscall"

	"github.com/sylabs/singularity/src/runtime/engines/kube"
)

// killInstance sends sig to instance identified by id and removes instance file is necessary.
func killInstance(id string, sig syscall.Signal) error {
	contInst, err := kube.GetInstance(id)
	if err == kube.ErrNotFound {
		return nil
	}
	if err != nil {
		return fmt.Errorf("could not read %q instance file: %v", id, err)
	}

	err = syscall.Kill(contInst.Pid, sig)
	if err != nil && err != syscall.ESRCH {
		return fmt.Errorf("could not send %v signal to %q: %v", sig, id, err)
	}
	log.Printf("%q is killed with %v", id, sig)

	// todo optimize this
	err = syscall.Kill(contInst.PPid, 0)
	for err == nil {
		err = syscall.Kill(contInst.PPid, 0)
	}
	if err != nil && err != syscall.ESRCH {
		return fmt.Errorf("could not kill monitor: %v", err)
	}
	log.Printf("monitor for %q is killed with %v", id, sig)
	return nil
}

func removeElem(a []string, v string) []string {
	for i := 0; i < len(a); i++ {
		if a[i] == v {
			a = append(a[:i], a[i+1:]...)
			i--
		}
	}
	return a
}

func addElem(a []string, v string) []string {
	for _, e := range a {
		if e == v {
			return a
		}
	}
	return append(a, v)
}
