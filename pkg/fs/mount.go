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
