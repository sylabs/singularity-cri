package image

import (
	"fmt"
	"strings"
)

type shubImageInfo struct {
	ref       string
	user      string
	container string
	tags      []string
}

func parseShubRef(ref string) (shubImageInfo, error) {
	ref = strings.TrimPrefix(ref, "shub://")
	refParts := strings.Split(ref, "/")

	if len(refParts) < 2 {
		return shubImageInfo{}, fmt.Errorf("not a valid shub reference")
	}

	info := shubImageInfo{
		ref:       "shub://" + ref,
		user:      refParts[len(refParts)-2],
		container: refParts[len(refParts)-1],
	}

	imageParts := strings.Split(info.container, `:`)
	if len(imageParts) != 1 {
		info.container = imageParts[0]
		info.tags = strings.Split(imageParts[1], ",")
	}

	return info, nil
}

func (i shubImageInfo) filename() string {
	return strings.Join([]string{i.user, i.container}, "_") + ".sif"
}
