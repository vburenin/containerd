package vicsnapshotter

import (
	"fmt"
	"io"
	"io/ioutil"
	"strings"

	"google.golang.org/grpc"

	"github.com/Sirupsen/logrus"
	"github.com/containerd/containerd/api/services/diff"
	diffapi "github.com/containerd/containerd/api/services/diff"
	"github.com/containerd/containerd/api/types/descriptor"
	mounttypes "github.com/containerd/containerd/api/types/mount"
	"github.com/containerd/containerd/archive/compression"
	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/plapi/client"
	"github.com/containerd/containerd/plapi/client/storage"
	"github.com/containerd/containerd/plapi/models"
	"github.com/containerd/containerd/plapi/vicconfig"
	"github.com/containerd/containerd/plapi/vicruntime"
	"github.com/containerd/containerd/plugin"
	"github.com/go-openapi/swag"
	digest "github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	context "golang.org/x/net/context"
)

func init() {
	plugin.Register("vicdiff-grpc", &plugin.Registration{
		Type: plugin.GRPCPlugin,
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

func (vd *VicDiffer) Register(gs *grpc.Server) error {
	diffapi.RegisterDiffServer(gs, vd)
	return nil
}

func (vd *VicDiffer) Apply(ctx context.Context, er *diff.ApplyRequest) (*diff.ApplyResponse, error) {
	desc := toDescriptor(er.Diff)
	// TODO: Check for supported media types

	mounts := toMounts(er.Mounts)
	if len(mounts) == 0 {
		return nil, errors.New("No mounts was given")
	}

	r, err := vd.store.Reader(ctx, desc.Digest)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get reader from content store")
	}
	defer r.Close()

	ds, err := compression.DecompressStream(r)
	digester := digest.Canonical.Digester()

	if err != nil {
		return nil, err
	}

	rc := &readCounter{
		r: io.TeeReader(ds, digester.Hash()),
	}

	img, err := vd.writeImage(ctx, r, desc.Digest.String(), mounts[0])
	logrus.Infof("returned image data: %q", img)

	// Read any trailing data.
	if _, err := io.Copy(ioutil.Discard, r); err != nil {
		return nil, err
	}
	if err != nil {
		return nil, err
	}

	resp := &diff.ApplyResponse{
		Applied: &descriptor.Descriptor{
			MediaType: ocispec.MediaTypeImageLayer,
			Digest:    digester.Digest(),
			Size_:     rc.c,
		},
	}

	logrus.Infof("Returning response: %q", resp)

	return resp, nil
}

func (vd *VicDiffer) Diff(ctx context.Context, dr *diff.DiffRequest) (*diff.DiffResponse, error) {
	return nil, errors.New("Snapshotter diff is not implemented")
}

func (vd *VicDiffer) writeImage(ctx context.Context, data io.Reader, sum string, m *VicMount) (*models.Image, error) {
	imgParams := storage.NewWriteImageParamsWithContext(ctx).
		WithImageID(m.current).
		WithParentID(m.parent).
		WithStoreName(vd.storageName).
		WithMetadatakey(swag.String("metaData")).
		WithMetadataval(swag.String("")).
		WithImageFile(ioutil.NopCloser(data)).
		WithSum(sum)
	r, err := vd.plClient.Storage.WriteImage(imgParams)
	if err != nil {
		return nil, err
	}
	return r.Payload, nil
}

func parseMount(mountSrc string) (*VicMount, error) {
	parts := strings.Split(mountSrc, "/")
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

func toMounts(apim []*mounttypes.Mount) []*VicMount {
	mounts := make([]*VicMount, len(apim))
	for i, m := range apim {
		vm, err := parseMount(m.Source)
		if err != nil {
			return nil
		}
		vm.target = m.Target
		vm.fs = m.Type
		vm.options = m.Options
		mounts[i] = vm
	}
	return mounts
}

func toDescriptor(d *descriptor.Descriptor) ocispec.Descriptor {
	return ocispec.Descriptor{
		MediaType: d.MediaType,
		Digest:    d.Digest,
		Size:      d.Size_,
	}
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
