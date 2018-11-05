package container

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"syscall"

	"github.com/sylabs/cri/pkg/image"
	"github.com/sylabs/sif/pkg/sif"
	"github.com/sylabs/singularity/src/pkg/util/loop"
)

const (
	containerInfoPath = "/var/run/singularity/containers/"

	bundleStorePath = "bundle/"
	rootfsStorePath = "rootfs/"
	ociConfigPath   = "config.json"
)

// ociConfigPath returns path to container's config.json file.
func (c *Container) ociConfigPath() string {
	return filepath.Join(containerInfoPath, c.id, bundleStorePath, ociConfigPath)
}

// rootfsPath returns path to container's rootfs directory.
func (c *Container) rootfsPath() string {
	return filepath.Join(containerInfoPath, c.id, bundleStorePath, rootfsStorePath)
}

func (c *Container) addOCIBundle(image *image.Info) error {
	err := os.MkdirAll(filepath.Join(containerInfoPath, c.id, bundleStorePath, rootfsStorePath), os.ModePerm)
	if err != nil {
		return fmt.Errorf("could not create rootfs directory for container: %v", err)
	}

	ociSpec, err := generateOCI(c, c.pod)
	if err != nil {
		return fmt.Errorf("could not generate oci spec for container: %v", err)
	}
	config, err := os.OpenFile(c.ociConfigPath(), os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("could not create OCI config file: %v", err)
	}
	defer config.Close()
	err = json.NewEncoder(config).Encode(ociSpec)
	if err != nil {
		return fmt.Errorf("could not encode OCI config into json: %v", err)
	}
	return mountImage(image.Path(), c.rootfsPath())
}

func (c *Container) cleanupBundle() error {
	return nil
}

func mountImage(imagePath, targetPath string) error {
	imageFile, err := os.Open(imagePath)
	if err != nil {
		return fmt.Errorf("could not open image: %v", err)
	}
	fimg, err := sif.LoadContainerFp(imageFile, true)
	if err != nil {
		return err
	}
	part, _, err := fimg.GetPartPrimSys()
	if err != nil {
		return err
	}
	fstype, err := part.GetFsType()
	if err != nil {
		return err
	}
	if fstype != sif.FsSquash {
		return fmt.Errorf("unsuported image fs type: %v", fstype)
	}
	loopFlags := uint32(loop.FlagsAutoClear)
	info := loop.Info64{
		Offset:    uint64(part.Fileoff),
		SizeLimit: uint64(part.Filelen),
		Flags:     loopFlags,
	}
	if err := imageFile.Close(); err != nil {
		return fmt.Errorf("could not close file: %v", err)
	}

	log.Printf("mounting %s into loop device", imagePath)
	dev, err := loopDevice(imagePath, os.O_RDWR, &info)
	if err != nil {
		return fmt.Errorf("could not attach loop dev: %v", err)
	}

	log.Printf("mounting loop device #%d into %s", dev, targetPath)
	err = syscall.Mount(fmt.Sprintf("/dev/loop%d", dev), targetPath, "squashfs", syscall.MS_NOSUID|syscall.MS_REC, "")
	if err != nil {
		return fmt.Errorf("could not mount loop device: %v", err)
	}
	return nil
}

func loopDevice(path string, mode int, info *loop.Info64) (int, error) {
	var devNum int
	var loopdev loop.Device
	loopdev.MaxLoopDevices = 256
	loopdev.AttachFromPath(path, mode, &devNum)
	return devNum, loopdev.SetStatus(info)
}
