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

package fs

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

type (
	// Device contains information about a loop device.
	Device struct {
		Device         uint64
		Inode          uint64
		Rdevice        uint64
		Offset         uint64
		SizeLimit      uint64
		Number         uint32
		EncryptType    uint32
		EncryptKeySize uint32
		Flags          uint32
		FileName       [64]byte
		CryptName      [64]byte
		EncryptKey     [32]byte
		Init           [2]uint64
	}
)

func DeviceInfo(path string) (*Device, error) {
	device, err := os.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		return nil, fmt.Errorf("could not open device: %v", err)
	}
	defer device.Close()

	var info Device
	_, _, errNo := syscall.Syscall(syscall.SYS_IOCTL, device.Fd(), 0x4C05, uintptr(unsafe.Pointer(&info)))
	if errNo != syscall.ENXIO && errNo != 0 {
		return nil, fmt.Errorf("could not get loop device info: %v", syscall.Errno(errNo))
	}

	return &info, nil
}
