package mounts

import (
	"fmt"
	"strings"

	"crypto/sha256"

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
	parts := strings.Split(src, "_")
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

func HashKey(part string) string {
	h := sha256.New()
	h.Write([]byte(part))
	return fmt.Sprintf("%x", h.Sum(nil))
}

func HashParent(part string) string {
	if part == "scratch" || part == "" {
		return "scratch"
	}
	return HashKey(part)
}

func ParseMount(m mount.Mount) (*VicMount, error) {
	return ParseMountSource(m.Source)

}

func FormatMountSource(snapType, key, parent string) string {
	parent = HashParent(parent)
	key = HashKey(key)
	return fmt.Sprintf("%s_%s_%s", snapType, parent, key)
}
