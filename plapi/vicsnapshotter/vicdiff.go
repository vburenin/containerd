package vicsnapshotter

import (
	"io"
	"io/ioutil"

	"github.com/Sirupsen/logrus"
	"github.com/containerd/containerd/archive/compression"
	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/mount"
	"github.com/containerd/containerd/plapi/client"
	"github.com/containerd/containerd/plapi/client/storage"
	"github.com/containerd/containerd/plapi/mounts"
	"github.com/containerd/containerd/plapi/vicconfig"
	"github.com/containerd/containerd/plapi/vicruntime"
	"github.com/containerd/containerd/plugin"
	"github.com/containerd/containerd/snapshot"
	"github.com/go-openapi/swag"
	digest "github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
)

func init() {
	plugin.Register(&plugin.Registration{
		ID:     "diff-vic",
		Type:   plugin.DiffPlugin,
		Init:   NewVicDiffer,
		Config: vicconfig.DefaultConfig(),
		Requires: []plugin.PluginType{
			plugin.ContentPlugin,
			plugin.SnapshotPlugin,
		},
	})
}

type VicDiffer struct {
	store       content.Store
	storageName string
	plClient    *client.PortLayer
	shotter     snapshot.Snapshotter
}

var emptyDesc = ocispec.Descriptor{}

func NewVicDiffer(ic *plugin.InitContext) (interface{}, error) {
	cfg := ic.Config.(*vicconfig.Config)
	c, err := ic.Get(plugin.ContentPlugin)
	if err != nil {
		return nil, err
	}
	s, err := ic.Get(plugin.SnapshotPlugin)
	if err != nil {
		return nil, err
	}
	return &VicDiffer{
		store:       c.(content.Store),
		plClient:    vicruntime.PortLayerClient(cfg.PortlayerAddress),
		storageName: "containerd-storage",
		shotter:     s.(snapshot.Snapshotter),
	}, nil
}

func (vd *VicDiffer) Apply(ctx context.Context, desc ocispec.Descriptor, mnts []mount.Mount) (ocispec.Descriptor, error) {

	if len(mnts) == 0 {
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

	vicMount, err := mounts.ParseMount(mnts[0])
	if err != nil {
		return emptyDesc, err
	}

	checkSum, err := vd.writeImage(ctx, ds, vicMount.Current, vicMount)

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

func (vd *VicDiffer) writeImage(ctx context.Context, data io.Reader, sum string, m *mounts.VicMount) (string, error) {
	stat, err := vd.shotter.Stat(ctx, m.Current)
	if err != nil {
		return "", err
	}
	params := storage.NewUnpackImageParamsWithContext(ctx).
		WithImageID(m.Current).
		WithStoreName(vd.storageName).
		WithParentID(m.Parent).
		WithMetadatakey(swag.String("orig_name")).
		WithMetadataval(&stat.Name).
		WithImageFile(ioutil.NopCloser(data))

	r, err := vd.plClient.Storage.UnpackImage(params)
	if err != nil {
		return "", err
	}
	return r.Payload, nil
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
