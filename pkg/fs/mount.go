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
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// MountPoint parses /proc/self/mountinfo and returns the path of the parent
// mount point where provided path is mounted in.
func MountPoint(path string) (string, error) {
	const (
		mountInfoPath = "/proc/self/mountinfo"
		defaultRoot   = "/"
	)

	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", fmt.Errorf("could not resolve path %s: %v", path, err)
	}

	p, err := os.Open(mountInfoPath)
	if err != nil {
		return "", fmt.Errorf("could not open %s: %v", mountInfoPath, err)
	}
	defer p.Close()

	var mountPoints []string
	scanner := bufio.NewScanner(p)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		mountPoints = append(mountPoints, fields[4])
	}

	for resolved != defaultRoot {
		for _, point := range mountPoints {
			if point == resolved {
				return point, nil
			}
		}
		resolved = filepath.Dir(resolved)
	}

	return defaultRoot, nil
}
