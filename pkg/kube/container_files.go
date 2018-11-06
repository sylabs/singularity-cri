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

package kube

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
)

// ociConfigPath returns path to container's config.json file.
func (c *Container) ociConfigPath() string {
	return filepath.Join(containerInfoPath, c.id, bundleStorePath, ociConfigPath)
}

// RootfsPath returns path to container's rootfs directory.
func (c *Container) RootfsPath() string {
	return rootfsStorePath
	//return filepath.Join(containerInfoPath, c.id, bundleStorePath, rootfsStorePath)
}

// socketPath returns path to contaienr's sync socket.
func (c *Container) socketPath() string {
	return filepath.Join(containerInfoPath, c.id, socketPath)
}

// bundlePath returns path to container's filesystem bundle directory.
func (c *Container) bundlePath() string {
	return filepath.Join(containerInfoPath, c.id, bundleStorePath)
}

func (c *Container) addOCIBundle(image *image.Info) error {
	err := os.MkdirAll(filepath.Join(containerInfoPath, c.id, bundleStorePath, rootfsStorePath), os.ModePerm)
	if err != nil {
		return fmt.Errorf("could not create rootfs directory for container: %v", err)
	}

	ociSpec, err := translateContainer(c, c.pod)
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
	return mountImage(image.Path(), filepath.Join(containerInfoPath, c.id, bundleStorePath, rootfsStorePath))
	//return mountImage(image.Path(), c.RootfsPath())
}

func (c *Container) cleanupFiles(silent bool) error {
	//err := syscall.Unmount(c.RootfsPath(), 0)
	err := syscall.Unmount(filepath.Join(containerInfoPath, c.id, bundleStorePath, rootfsStorePath), 0)
	if err != nil && !silent {
		return fmt.Errorf("could not umount rootfs: %v", err)
	}
	//err = os.RemoveAll(filepath.Join(containerInfoPath, c.id))
	//if err != nil && !silent {
	//	return fmt.Errorf("could not cleanup container: %v", err)
	//}
	return nil
}

func mountImage(imagePath, targetPath string) error {
	imageFile, err := os.Open(imagePath)
	if err != nil {
		return fmt.Errorf("could not open image: %v", err)
	}
	defer func() {
		if err := imageFile.Close(); err != nil {
			log.Printf("could not close image file: %v", err)
		}
	}()

	fimg, err := sif.LoadContainerFp(imageFile, true)
	if err != nil {
		return fmt.Errorf("could not load image fp: %v", err)
	}
	part, _, err := fimg.GetPartPrimSys()
	if err != nil {
		return fmt.Errorf("could not get primaty partitions: %v", err)
	}
	fstype, err := part.GetFsType()
	if err != nil {
		return fmt.Errorf("could not get fs type: %v", err)
	}

	if fstype != sif.FsSquash {
		return fmt.Errorf("unsuported image fs type: %v", fstype)
	}
	info := &loop.Info64{
		Offset:    uint64(part.Fileoff),
		SizeLimit: uint64(part.Filelen),
		Flags:     uint32(loop.FlagsAutoClear),
	}

	var devNum int
	var loopdev loop.Device
	loopdev.MaxLoopDevices = 256
	err = loopdev.AttachFromFile(imageFile, os.O_RDWR, &devNum)
	if err != nil {
		return fmt.Errorf("could not attach image to loop device: %v", err)
	}
	err = loopdev.SetStatus(info)
	if err != nil {
		return fmt.Errorf("could not set loop device status: %v", err)
	}

	err = syscall.Mount(fmt.Sprintf("/dev/loop%d", devNum), targetPath, "squashfs", syscall.MS_NOSUID|syscall.MS_RDONLY, "errors=remount-ro")
	if err != nil {
		return fmt.Errorf("could not mount loop device: %v", err)
	}
	return nil
}
