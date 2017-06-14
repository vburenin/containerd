package vicsnapshotter

import (
	"fmt"
	"io"
	"io/ioutil"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/containerd/containerd/archive/compression"
	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/mount"
	"github.com/containerd/containerd/plapi/client"
	"github.com/containerd/containerd/plapi/client/storage"
	"github.com/containerd/containerd/plapi/vicconfig"
	"github.com/containerd/containerd/plapi/vicruntime"
	"github.com/containerd/containerd/plugin"
	"github.com/go-openapi/swag"
	digest "github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
)

func init() {
	plugin.Register("diff-vic", &plugin.Registration{
		Type: plugin.DiffPlugin,
		Init: func(ic *plugin.InitContext) (interface{}, error) {
			cfg := ic.Config.(*vicconfig.Config)
			return &VicDiffer{
				store:       ic.Content,
				plClient:    vicruntime.PortLayerClient(cfg.PortlayerAddress),
				storageName: "containerd-storage",
			}, nil
		},
		Config: vicconfig.DefaultConfig(),
	})
}

type VicDiffer struct {
	store       content.Store
	storageName string
	plClient    *client.PortLayer
}

var emptyDesc = ocispec.Descriptor{}

func (vd *VicDiffer) Apply(ctx context.Context, desc ocispec.Descriptor, mounts []mount.Mount) (ocispec.Descriptor, error) {

	if len(mounts) == 0 {
		return emptyDesc, errors.New("No mounts was given")
	}

	r, err := vd.store.Reader(ctx, desc.Digest)
	if err != nil {
		return emptyDesc, errors.Wrap(err, "failed to get reader from content store")
	}
	defer r.Close()

	ds, err := compression.DecompressStream(r)

	if err != nil {
		return emptyDesc, err
	}

	rc := &readCounter{r: ds}

	vicMount, err := parseMount(mounts[0])
	if err != nil {
		return emptyDesc, err
	}

	checkSum, err := vd.writeImage(ctx, ds, vicMount.current, vicMount)

	// Read any trailing data.
	if _, err := io.Copy(ioutil.Discard, r); err != nil {
		return emptyDesc, err
	}
	if err != nil {
		return emptyDesc, err
	}

	resp := ocispec.Descriptor{
		MediaType: ocispec.MediaTypeImageLayer,
		Digest:    digest.Digest(checkSum),
		Size:      rc.c,
	}

	logrus.Infof("Returning response: %q", resp)

	return resp, nil
}

func (vd *VicDiffer) DiffMounts(ctx context.Context, lower, upper []mount.Mount, media, ref string) (ocispec.Descriptor, error) {
	return ocispec.Descriptor{}, errors.New("Snapshotter diff is not implemented")
}

func (vd *VicDiffer) writeImage(ctx context.Context, data io.Reader, sum string, m *VicMount) (string, error) {
	params := storage.NewUnpackImageParamsWithContext(ctx).
		WithImageID(m.current).
		WithStoreName(vd.storageName).
		WithParentID(m.parent).
		WithMetadatakey(swag.String("metaData")).
		WithMetadataval(swag.String("")).
		WithImageFile(ioutil.NopCloser(data))

	r, err := vd.plClient.Storage.UnpackImage(params)
	if err != nil {
		return "", err
	}
	return r.Payload, nil
}

func parseMount(m mount.Mount) (*VicMount, error) {
	mountSrc := m.Source
	parts := strings.Split(m.Source, "/")
	if len(parts) != 3 {
		return nil, fmt.Errorf("Invalid source point: %s", mountSrc)
	}
	if parts[0] != "view" && parts[0] != "img" {
		return nil, fmt.Errorf("Invalid mount type: %s", mountSrc)
	}

	return &VicMount{
		parent:  parts[1],
		current: parts[2],
	}, nil
}

type readCounter struct {
	r io.Reader
	c int64
}

func (rc *readCounter) Read(p []byte) (n int, err error) {
	n, err = rc.r.Read(p)
	rc.c += int64(n)
	return
}
