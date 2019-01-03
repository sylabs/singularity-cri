package fs

import (
	"fmt"
	"os"
	"path/filepath"
)

// UsageInfo holds metrics on fs usage.
type UsageInfo struct {
	MountPoint string
	Bytes      int64
	Inodes     int64
}

// Usage collects fs usage for specific location, often a directory.
func Usage(path string) (*UsageInfo, error) {
	mount, err := MountPoint(path)
	if err != nil {
		return nil, fmt.Errorf("could not get mount point: %v", err)
	}

	bytes, inodes, err := fetchStat(path)
	if err != nil {
		return nil, fmt.Errorf("could not fetch fs stat: %v", err)
	}

	return &UsageInfo{
		MountPoint: mount,
		Bytes:      bytes,
		Inodes:     inodes,
	}, nil
}

func fetchStat(path string) (int64, int64, error) {
	storeDir, err := os.Open(path)
	if err != nil {
		return 0, 0, fmt.Errorf("could not open %q: %v", path, err)
	}
	defer storeDir.Close()

	fi, err := storeDir.Stat()
	if err != nil {
		return 0, 0, fmt.Errorf("could not get info for %q: %v", path, err)
	}

	fii, err := storeDir.Readdir(-1)
	if err != nil {
		return 0, 0, fmt.Errorf("could not scan %q: %v", path, err)
	}

	var bytes int64
	var inodes int64
	for _, fi := range fii {
		if fi.IsDir() {
			b, i, err := fetchStat(filepath.Join(path, fi.Name()))
			if err != nil {
				return 0, 0, fmt.Errorf("could not fetch info: %v", err)
			}
			bytes += b
			inodes += i
		} else {
			bytes += fi.Size()
			inodes++
		}
	}
	// add directory info as well
	inodes++
	bytes += fi.Size()

	return bytes, inodes, nil
}
