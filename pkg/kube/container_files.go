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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/golang/glog"
	"github.com/sylabs/sif/pkg/sif"
	"github.com/sylabs/singularity/pkg/util/loop"
	k8s "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
)

const (
	contInfoPath = "/var/lib/singularity/sycri/containers/"

	contSocketPath    = "sync.sock"
	contBundlePath    = "bundle/"
	contRootfsPath    = "rootfs/"
	contOCIConfigPath = "config.json"

	fakeShPath = "/usr/local/bin/sycri-bin/fakesh"
)

func (c *Container) baseDir() string {
	return filepath.Join(contInfoPath, c.id)
}

// ociConfigPath returns path to container's config.json file.
func (c *Container) ociConfigPath() string {
	return filepath.Join(c.baseDir(), contBundlePath, contOCIConfigPath)
}

// rootfsPath returns path to container's rootfs directory.
func (c *Container) rootfsPath() string {
	return filepath.Join(c.baseDir(), contBundlePath, contRootfsPath)
}

// socketPath returns path to container's sync socket.
func (c *Container) socketPath() string {
	return filepath.Join(c.baseDir(), contSocketPath)
}

// bundlePath returns path to container's filesystem bundle directory.
func (c *Container) bundlePath() string {
	return filepath.Join(c.baseDir(), contBundlePath)
}

func (c *Container) addLogDirectory() error {
	logDir := c.pod.GetLogDirectory()
	logPath := c.GetLogPath()
	if logDir == "" || logPath == "" {
		return nil
	}

	logPath = filepath.Join(logDir, logPath)
	logDir = filepath.Dir(logPath)
	glog.V(8).Infof("Creating log directory %s", logDir)
	err := os.MkdirAll(logDir, 0755)
	if err != nil {
		return fmt.Errorf("could not create %s: %v", logDir, err)
	}
	c.logPath = logPath
	return nil
}

func (c *Container) addOCIBundle() error {
	glog.V(8).Infof("Creating bundle directory %s", c.bundlePath())
	err := os.MkdirAll(c.bundlePath(), 0755)
	if err != nil {
		return fmt.Errorf("could not create bundle directory for container: %v", err)
	}
	if err := c.prepareOverlay(c.imgInfo.Path()); err != nil {
		return err
	}
	if err := c.ensureSh(); err != nil {
		return err
	}
	ociSpec, err := translateContainer(c, c.pod)
	if err != nil {
		return fmt.Errorf("could not generate oci spec for container: %v", err)
	}
	glog.V(8).Infof("Creating oci config %s", c.ociConfigPath())
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
		overlayDir = filepath.Join(c.bundlePath(), "overlay")
		lowerPath  = filepath.Join(c.bundlePath(), "lower")
		upperPath  = filepath.Join(overlayDir, "upper")
		workPath   = filepath.Join(overlayDir, "work")
	)

	// prepare upper and work directories
	glog.V(8).Infof("Creating %s", overlayDir)
	err := os.Mkdir(overlayDir, 0755)
	if err != nil {
		return fmt.Errorf("could not create overlay parent directory: %v", err)
	}
	glog.V(8).Infof("Creating %s", upperPath)
	err = os.Mkdir(upperPath, 0755)
	if err != nil {
		return fmt.Errorf("could not create upper directory for overlay: %v", err)
	}
	glog.V(8).Infof("Creating %s", workPath)
	err = os.Mkdir(workPath, 0755)
	if err != nil {
		return fmt.Errorf("could not create working directory for overlay: %v", err)
	}
	glog.V(8).Infof("Bind mounting %s", overlayDir)
	err = syscall.Mount(overlayDir, overlayDir, "", syscall.MS_BIND, "")
	if err != nil {
		return fmt.Errorf("could not bind mount overlay parent directory: %v", err)
	}
	glog.V(8).Infof("Remounting %s to enable suid flow", overlayDir)
	err = syscall.Mount(overlayDir, overlayDir, "", syscall.MS_REMOUNT|syscall.MS_BIND, "")
	if err != nil {
		return fmt.Errorf("could not remount overlay parent directory: %v", err)
	}

	// prepare image
	glog.V(8).Infof("Creating %s", lowerPath)
	err = os.Mkdir(lowerPath, 0755)
	if err != nil {
		return fmt.Errorf("could not create lower directory for overlay: %v", err)
	}
	err = mountImage(imagePath, lowerPath)
	if err != nil {
		return fmt.Errorf("could not mount image: %v", err)
	}

	// merge all together
	glog.V(8).Infof("Creating %s", c.rootfsPath())
	err = os.Mkdir(c.rootfsPath(), 0755)
	if err != nil {
		return fmt.Errorf("could not create rootfs directory: %v", err)
	}
	overlayOpts := fmt.Sprintf("lowerdir=%s,workdir=%s,upperdir=%s", lowerPath, workPath, upperPath)
	glog.V(8).Infof("Mounting overlay at %s with options: %v", c.rootfsPath(), overlayOpts)
	err = syscall.Mount("overlay", c.rootfsPath(), "overlay", 0, overlayOpts)
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
			glog.Errorf("Could not close image file: %v", err)
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

	loopDev := loop.Device{
		MaxLoopDevices: 256,
		Info: &loop.Info64{
			Offset:    uint64(part.Fileoff),
			SizeLimit: uint64(part.Filelen),
			Flags:     uint32(loop.FlagsAutoClear),
		},
	}
	var devNum int
	err = loopDev.AttachFromFile(imageFile, os.O_RDWR, &devNum)
	if err != nil {
		return fmt.Errorf("could not attach image to loop device: %v", err)
	}
	glog.V(8).Infof("Attached %s to loop device #%d", imagePath, devNum)

	glog.V(8).Infof("Mounting loop device #%d to %s", devNum, targetPath)
	err = syscall.Mount(fmt.Sprintf("/dev/loop%d", devNum), targetPath,
		"squashfs", syscall.MS_RDONLY, "errors=remount-ro")
	if err != nil {
		return fmt.Errorf("could not mount loop device: %v", err)
	}
	return nil
}

func (c *Container) cleanupFiles(silent bool) error {
	glog.V(8).Infof("Unmounting overlay from %s ", c.rootfsPath())
	err := syscall.Unmount(c.rootfsPath(), 0)
	if err != nil && !silent {
		return fmt.Errorf("could not umount rootfs: %v", err)
	}
	lowerPath := filepath.Join(c.bundlePath(), "lower")
	glog.V(8).Infof("Unmounting image from %s ", lowerPath)
	err = syscall.Unmount(lowerPath, 0)
	if err != nil && !silent {
		return fmt.Errorf("could not umount image: %v", err)
	}
	overlayPath := filepath.Join(c.bundlePath(), "overlay")
	glog.V(8).Infof("Unmounting overlay directory from %s ", overlayPath)
	err = syscall.Unmount(overlayPath, 0)
	if err != nil && !silent {
		return fmt.Errorf("could not umount overlay parent directory: %v", err)
	}
	glog.V(8).Infof("Removing container base directory %s", c.baseDir())
	err = os.RemoveAll(c.baseDir())
	if err != nil && !silent {
		return fmt.Errorf("could not cleanup container: %v", err)
	}
	if c.logPath != "" {
		dir := filepath.Dir(c.logPath)
		// in case container's logs are not stored separately
		if dir != c.pod.GetLogDirectory() {
			glog.V(8).Infof("Removing container log directory %s", dir)
			err = os.RemoveAll(dir)
			if err != nil && !silent {
				return fmt.Errorf("could not remove logs: %v", err)
			}
		}
	}
	return nil
}

func (c *Container) ensureSh() error {
	_, err := os.Stat(filepath.Join(c.rootfsPath(), "bin", "sh"))
	if os.IsNotExist(err) {
		c.Mounts = append(c.Mounts, &k8s.Mount{
			ContainerPath: "/bin/sh",
			HostPath:      fakeShPath,
		})
		return nil
	}
	if err != nil {
		return fmt.Errorf("could not check contaiener shell")
	}
	return nil
}
