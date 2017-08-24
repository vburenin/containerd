package vicsnapshotter

import (
	"io"
	"io/ioutil"

	"github.com/boltdb/bolt"
	"github.com/containerd/containerd/archive/compression"
	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/metadata"
	"github.com/containerd/containerd/mount"
	"github.com/containerd/containerd/plapi/client"
	"github.com/containerd/containerd/plapi/client/storage"
	"github.com/containerd/containerd/plapi/mounts"
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
	log.G(context.Background()).Infof("Registering VIC Diff")
	plugin.Register(&plugin.Registration{
		ID:     "diff-vic",
		Type:   plugin.DiffPlugin,
		Init:   NewVicDiffer,
		Config: vicconfig.DefaultConfig(),
		Requires: []plugin.PluginType{
			plugin.ContentPlugin,
			plugin.SnapshotPlugin,
			plugin.MetadataPlugin,
		},
	})
}

type VicDiffer struct {
	store       content.Store
	storageName string
	plClient    *client.PortLayer
	shotter     *VicSnap
}

var emptyDesc = ocispec.Descriptor{}

func NewVicDiffer(ic *plugin.InitContext) (interface{}, error) {
	cfg := ic.Config.(*vicconfig.Config)
	c, err := ic.Get(plugin.ContentPlugin)
	if err != nil {
		return nil, err
	}
	snapshotters, err := ic.GetAll(plugin.SnapshotPlugin)
	if err != nil {
		return nil, err
	}

	s, ok := snapshotters["snapshot-vic"]
	if !ok {
		return nil, errors.Wrap(errdefs.ErrNotFound, "VicDiffer requires Vic Snapshotter")
	}

	md, err := ic.Get(plugin.MetadataPlugin)
	if err != nil {
		return nil, err
	}

	cstore := metadata.NewContentStore(md.(*bolt.DB), c.(content.Store))

	return &VicDiffer{
		store:       cstore,
		plClient:    vicruntime.PortLayerClient(cfg.PortlayerAddress),
		storageName: "containerd-storage",
		shotter:     s.(*VicSnap),
	}, nil
}

func (vd *VicDiffer) Apply(ctx context.Context, desc ocispec.Descriptor, mnts []mount.Mount) (ocispec.Descriptor, error) {
	log.G(ctx).Debugf("Applying descriptor: %s", desc.Digest)
	if len(mnts) == 0 {
		return emptyDesc, errors.Wrap(errdefs.ErrInvalidArgument, "No mounts was given")
	}
	rat, err := vd.store.ReaderAt(ctx, desc.Digest)
	r := content.NewReader(rat)
	if err != nil {
		return emptyDesc, errors.Wrap(err, "failed to get reader from content store")
	}
	defer rat.Close()

	ds, err := compression.DecompressStream(r)

	if err != nil {
		return emptyDesc, errors.Wrap(err, "Could not decompress stream")
	}

	rc := &readCounter{r: ds}

	vicMount, err := mounts.ParseMount(mnts[0])
	if err != nil {
		return emptyDesc, errors.Wrap(err, "failed to parse mounts")
	}

	checkSum, err := vd.writeImage(ctx, ds, vicMount.Current, vicMount)

	if err != nil {
		log.G(ctx).WithError(err).Error("Could not store image")
		return emptyDesc, errors.Wrap(err, "Failed to write image")
	}

	log.G(ctx).Debugf("Image checksum: %s", checkSum)
	// Read any trailing data.
	if _, err := io.Copy(ioutil.Discard, r); err != nil {
		return emptyDesc, errors.Wrap(err, "Failed to discard not needed data")
	}

	resp := ocispec.Descriptor{
		MediaType: ocispec.MediaTypeImageLayer,
		Digest:    digest.Digest(checkSum),
		Size:      rc.c,
	}

	log.G(ctx).Infof("Returning response: %q", resp.Digest)

	return resp, nil
}

func (vd *VicDiffer) DiffMounts(ctx context.Context, lower, upper []mount.Mount, media, ref string) (ocispec.Descriptor, error) {
	return ocispec.Descriptor{}, errors.New("Snapshotter diff is not implemented")
}

func (vd *VicDiffer) writeImage(ctx context.Context, data io.Reader, sum string, m *mounts.VicMount) (string, error) {
	stat, err := vd.shotter.StatByHash(ctx, m.Current)
	if err != nil {
		log.G(ctx).WithError(err).Error("Could not stat the data")
		return "", err
	}

	siBin, _ := dumpSnapInfo(&stat)
	params := storage.NewUnpackImageParamsWithContext(ctx).
		WithImageID(m.Current).
		WithStoreName(vd.storageName).
		WithParentID(m.Parent).
		WithMetadatakey(swag.String(PortLayerContainerMetadataKey)).
		WithMetadataval(&siBin).
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
