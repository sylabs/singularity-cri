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
	"github.com/sylabs/singularity/pkg/util/loop"
)

const (
	contInfoPath = "/var/run/singularity/containers/"

	contSocketPath    = "sync.sock"
	contBundlePath    = "bundle/"
	contRootfsPath    = "rootfs/"
	contOCIConfigPath = "config.json"
)

// ociConfigPath returns path to container's config.json file.
func (c *Container) ociConfigPath() string {
	return filepath.Join(contInfoPath, c.id, contBundlePath, contOCIConfigPath)
}

// rootfsPath returns path to container's rootfs directory.
func (c *Container) rootfsPath() string {
	return filepath.Join(contInfoPath, c.id, contBundlePath, contRootfsPath)
}

// socketPath returns path to container's sync socket.
func (c *Container) socketPath() string {
	return filepath.Join(contInfoPath, c.id, contSocketPath)
}

// bundlePath returns path to container's filesystem bundle directory.
func (c *Container) bundlePath() string {
	return filepath.Join(contInfoPath, c.id, contBundlePath)
}

func (c *Container) addLogDirectory() error {
	logDir := c.pod.GetLogDirectory()
	logPath := c.GetLogPath()
	if logDir == "" || logPath == "" {
		return nil
	}

	logPath = filepath.Join(logDir, logPath)
	logDir = filepath.Dir(logPath)
	err := os.MkdirAll(logDir, 0755)
	if err != nil {
		return fmt.Errorf("could not create %s: %v", logDir, err)
	}
	c.logPath = logPath
	return nil
}

func (c *Container) addOCIBundle(image *image.Info) error {
	err := os.MkdirAll(c.bundlePath(), 0755)
	if err != nil {
		return fmt.Errorf("could not create rootfs directory for container: %v", err)
	}
	if err := c.prepareOverlay(image.Path()); err != nil {
		return err
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
	return nil
}

func (c *Container) prepareOverlay(imagePath string) error {
	var (
		lowerPath = filepath.Join(c.bundlePath(), "lower")
		upperPath = filepath.Join(c.bundlePath(), "upper")
		workPath  = filepath.Join(c.bundlePath(), "work")
	)

	log.Printf("creating %s", lowerPath)
	err := os.Mkdir(lowerPath, 0755)
	if err != nil {
		return fmt.Errorf("could not create lower directory for overlay: %v", err)
	}
	log.Printf("creating %s", upperPath)
	err = os.Mkdir(upperPath, 0755)
	if err != nil {
		return fmt.Errorf("could not create upper directory for overlay: %v", err)
	}
	log.Printf("creating %s", workPath)
	err = os.Mkdir(workPath, 0755)
	if err != nil {
		return fmt.Errorf("could not create working directory for overlay: %v", err)
	}

	log.Printf("creating %s", c.rootfsPath())
	err = os.Mkdir(c.rootfsPath(), 0755)
	if err != nil {
		return fmt.Errorf("could not create root directory for overlay: %v", err)
	}

	err = mountImage(imagePath, lowerPath)
	if err != nil {
		return fmt.Errorf("could not mount image: %v", err)
	}

	overlayOpts := fmt.Sprintf("lowerdir=%s,workdir=%s,upperdir=%s", lowerPath, workPath, upperPath)
	log.Printf("mounting overlay with options: %v", overlayOpts)
	err = syscall.Mount("overlay", c.rootfsPath(), "overlay", syscall.MS_NOSUID|syscall.MS_REC, overlayOpts)
	if err != nil {
		return fmt.Errorf("could not mount overlay: %v", err)
	}
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

	err = syscall.Mount(fmt.Sprintf("/dev/loop%d", devNum), targetPath,
		"squashfs", syscall.MS_NOSUID|syscall.MS_RDONLY, "errors=remount-ro")
	if err != nil {
		return fmt.Errorf("could not mount loop device: %v", err)
	}
	return nil
}

func (c *Container) cleanupFiles(silent bool) error {
	err := syscall.Unmount(filepath.Join(c.bundlePath(), "lower"), 0)
	if err != nil && !silent {
		return fmt.Errorf("could not umount image: %v", err)
	}
	err = syscall.Unmount(c.rootfsPath(), 0)
	if err != nil && !silent {
		return fmt.Errorf("could not umount rootfs: %v", err)
	}
	err = os.RemoveAll(filepath.Join(contInfoPath, c.id))
	if err != nil && !silent {
		return fmt.Errorf("could not cleanup container: %v", err)
	}
	if c.logPath != "" {
		dir := filepath.Dir(c.logPath)
		// in case container's logs are not stored separately
		if dir != c.pod.GetLogDirectory() {
			err = os.RemoveAll(dir)
			if err != nil && !silent {
				return fmt.Errorf("could not remove logs: %v", err)
			}
		}
	}
	return nil
}
