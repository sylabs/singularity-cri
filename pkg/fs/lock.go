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
)

// Lock applies an exclusive lock on path.
func Lock(path string) (int, error) {
	f, err := os.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		return 0, fmt.Errorf("could not open %s: %v", path, err)
	}
	fd := int(f.Fd())
	err = syscall.Flock(fd, syscall.LOCK_EX)
	if err != nil {
		f.Close()
		return 0, fmt.Errorf("could not lock %s: %v", path, err)
	}
	return fd, nil
}

// Release removes a lock on path referenced by fd.
func Release(fd int) error {
	defer syscall.Close(fd)
	if err := syscall.Flock(fd, syscall.LOCK_UN); err != nil {
		return err
	}
	return nil
}
