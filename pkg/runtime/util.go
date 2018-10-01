package runtime

import (
	"fmt"
	"log"
	"os"
	"syscall"

	"github.com/sylabs/singularity/src/runtime/engines/kube"
)

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

	err = syscall.Kill(contInst.PPid, 0)
	// todo optimize this
	for err == nil {
		err = syscall.Kill(contInst.PPid, 0)
	}
	if err != nil && err != syscall.ESRCH {
		return fmt.Errorf("could not kill monitor: %v", err)
	}
	log.Printf("monitor for %q is killed with %v", id, sig)

	err = contInst.Delete()
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("could not remove %q instance file: %v", id, err)
	}
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
