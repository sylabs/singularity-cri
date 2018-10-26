package image

import (
	"fmt"

	"github.com/sylabs/cri/pkg/singularity"
	"github.com/sylabs/sif/pkg/sif"
	"github.com/sylabs/singularity/src/pkg/signing"
)

func verify(path string) error {
	fimg, err := sif.LoadContainer(path, true)
	if err != nil {
		return fmt.Errorf("failed to load SIF image: %v", err)
	}
	defer fimg.UnloadContainer()

	for _, desc := range fimg.DescrArr {
		err := signing.Verify(path, singularity.KeysServer, desc.ID, false, "")
		if err != nil {
			return fmt.Errorf("SIF verification failed: %v", err)
		}
	}
	return nil
}
