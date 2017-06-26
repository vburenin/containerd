package mounts

import (
	"fmt"
	"strings"

	"github.com/containerd/containerd/mount"
)

type VicMount struct {
	Parent   string
	Fs       string
	Current  string
	SnapType string
	Target   string
	Options  []string
}

func ParseMountSource(src string) (*VicMount, error) {
	parts := strings.Split(src, "/")
	if len(parts) != 3 {
		return nil, fmt.Errorf("Invalid source point: %s", src)
	}
	if parts[0] != "view" && parts[0] != "img" {
		return nil, fmt.Errorf("Invalid mount type: %s", src)
	}

	return &VicMount{
		Parent:  parts[1],
		Current: parts[2],
	}, nil
}

func ParseMount(m mount.Mount) (*VicMount, error) {
	return ParseMountSource(m.Source)

}
