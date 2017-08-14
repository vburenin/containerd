package vicsnapshotter

import (
	"context"
	"sync"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/mount"
	"github.com/containerd/containerd/plapi/client"
	"github.com/containerd/containerd/plapi/client/storage"
	"github.com/containerd/containerd/plapi/models"
	"github.com/containerd/containerd/plapi/mounts"
	"github.com/containerd/containerd/plapi/vicconfig"
	"github.com/containerd/containerd/plapi/vicruntime"
	"github.com/containerd/containerd/plugin"
	"github.com/containerd/containerd/snapshot"
	"github.com/pkg/errors"
)

func init() {
	plugin.Register(&plugin.Registration{
		ID:       "snapshot-vic",
		Type:     plugin.SnapshotPlugin,
		Init:     NewVicSnap,
		Config:   vicconfig.DefaultConfig(),
		Requires: []plugin.PluginType{},
	})
}

type VicSnap struct {
	storageName string
	plClient    *client.PortLayer
	snapshots   map[string]snapshot.Info
	active      map[string][]mount.Mount
	store       content.Store

	mu sync.Mutex
}

func NewVicSnap(ic *plugin.InitContext) (interface{}, error) {
	cfg := ic.Config.(*vicconfig.Config)
	vs := &VicSnap{
		storageName: "containerd-storage",
		plClient:    vicruntime.PortLayerClient(cfg.PortlayerAddress),
		snapshots:   make(map[string]snapshot.Info),
		active:      make(map[string][]mount.Mount),
	}
	if err := vs.createStorage(ic.Context); err != nil {
		return nil, err
	}
	if err := vs.loadAvailableImages(ic.Context); err != nil {
		return nil, err
	}
	log.G(ic.Context).Debug("VIC snapshotter has been initialized")
	return vs, nil
}

var _ snapshot.Snapshotter = &VicSnap{}

func (vs *VicSnap) createStorage(ctx context.Context) error {
	is := models.ImageStore{
		Name: vs.storageName,
	}

	r, err := vs.plClient.Storage.CreateImageStore(
		storage.NewCreateImageStoreParamsWithContext(ctx).WithBody(&is),
	)
	if err != nil {
		if _, ok := err.(*storage.CreateImageStoreConflict); !ok {
			return err
		}
	} else {
		log.G(ctx).Debugf("Storage has been created: %s", r.Payload.URL)
	}

	return nil
}

func (vs *VicSnap) loadAvailableImages(ctx context.Context) error {
	params := storage.NewListAllImagesParamsWithContext(ctx).WithStoreName(vs.storageName)
	resp, err := vs.plClient.Storage.ListAllImages(params)
	if err != nil {
		return err
	}
	log.G(ctx).Infof("Found %d images", len(resp.Payload))
	for _, img := range resp.Payload {
		kind := snapshot.KindActive
		if _, ok := img.Metadata["committed"]; ok {
			kind = snapshot.KindCommitted
		}
		si := snapshot.Info{
			Parent: img.Metadata["parent"],
			Name:   img.Metadata["orig_name"],
			Kind:   kind,
		}
		if si.Name == "" {
			si.Name = img.ID
		}
		vs.snapshots[img.ID] = si
		if kind == snapshot.KindActive {
			vs.addActive("img", img.ID, img.Parent)
		}

		log.G(ctx).Debugf("Discovered image %#v", img)
	}
	return nil
}

func (vs *VicSnap) Stat(ctx context.Context, key string) (snapshot.Info, error) {
	vs.mu.Lock()
	defer vs.mu.Unlock()
	s, ok := vs.snapshots[key]
	if !ok {
		return snapshot.Info{}, errors.Wrapf(errdefs.ErrNotFound, "snapshot %v", key)

	}
	return s, nil
}

func (vs *VicSnap) Usage(ctx context.Context, key string) (snapshot.Usage, error) {
	_, err := vs.Stat(ctx, key)
	if err != nil {
		return snapshot.Usage{}, err
	}
	return snapshot.Usage{
		Inodes: 1024,
		Size:   1024 * 1024,
	}, nil
}

func (vs *VicSnap) Mounts(ctx context.Context, key string) ([]mount.Mount, error) {
	vs.mu.Lock()
	vs.mu.Unlock()
	keyID := mounts.HashKey(key)
	si, ok := vs.snapshots[keyID]
	if !ok {
		return nil, errors.Wrapf(errdefs.ErrNotFound, "snapshot %v", key)
	}
	if si.Kind != snapshot.KindActive {
		return nil, errors.Wrapf(errdefs.ErrFailedPrecondition, "snapshot %v", key)
	}
	mounts, ok := vs.active[keyID]
	if !ok {
		log.G(ctx).WithField("key", key).Errorf("No mounts found for the active snapshot")
		return nil, errors.Wrapf(errdefs.ErrNotFound, "active snapshot %v", key)
	}
	return mounts, nil
}

func (vs *VicSnap) addActive(snapType, key, parent string) []mount.Mount {
	m := mount.Mount{
		Options: []string{"rw"},
		Type:    "ext4",
		Source:  mounts.FormatMountSource(snapType, key, parent),
	}

	keyID := mounts.HashKey(key)
	parentID := mounts.HashParent(parent)

	vs.snapshots[keyID] = snapshot.Info{
		Name:   key,
		Parent: parentID,
		Kind:   snapshot.KindActive,
	}

	mountList := []mount.Mount{m}
	vs.active[keyID] = mountList
	return mountList
}

func (vs *VicSnap) prepareSnapshot(ctx context.Context, snapType, key, parent string) ([]mount.Mount, error) {
	keyID := mounts.HashKey(key)
	parentID := mounts.HashParent(parent)
	log.G(ctx).Debugf("Preparing snapshot %s:%s/%s, key: %s", snapType, parent, key, keyID)
	vs.mu.Lock()
	defer vs.mu.Unlock()

	if _, ok := vs.snapshots[keyID]; ok {
		return nil, errors.Wrapf(errdefs.ErrAlreadyExists, "snapshot %v", key)
	}

	if parentID != "scratch" {
		if p, ok := vs.snapshots[parentID]; !ok {
			return nil, errors.Wrapf(errdefs.ErrNotFound, "snapshot %v", key)
		} else {
			if p.Kind == snapshot.KindActive {
				return nil, errors.Wrapf(errdefs.ErrInvalidArgument, "snapshot %v", key)
			}
		}
	}

	return vs.addActive(snapType, key, parent), nil
}

func (vs *VicSnap) Prepare(ctx context.Context, key, parent string) ([]mount.Mount, error) {
	return vs.prepareSnapshot(ctx, "img", key, parent)
}

func (vs *VicSnap) View(ctx context.Context, key, parent string) ([]mount.Mount, error) {
	return vs.prepareSnapshot(ctx, "view", key, parent)
}

func (vs *VicSnap) Commit(ctx context.Context, name, key string) error {
	vs.mu.Lock()
	defer vs.mu.Unlock()
	keyID := mounts.HashKey(key)
	nameID := mounts.HashKey(name)

	s, ok := vs.snapshots[keyID]
	if !ok {
		return errors.Wrapf(errdefs.ErrNotFound, "snapshot %v", key)
	}
	if s.Kind != snapshot.KindActive {
		return errors.Wrapf(errdefs.ErrFailedPrecondition, "requested snapshot %v not active", key)
	}

	if _, ok := vs.snapshots[nameID]; ok {
		return errors.Wrapf(errdefs.ErrAlreadyExists, "snapshot %v", key)
	}

	params := storage.NewCommitImageParamsWithContext(ctx).
		WithStoreName(vs.storageName).
		WithNewID(nameID).
		WithOldID(keyID).
		WithOrigName(name)

	_, err := vs.plClient.Storage.CommitImage(params)
	if err == nil {
		vs.snapshots[nameID] = snapshot.Info{
			Name:   name,
			Parent: s.Parent,
			Kind:   snapshot.KindCommitted,
		}
	}
	return err
}

func (vs *VicSnap) Remove(ctx context.Context, key string) error {
	vs.mu.Lock()
	defer vs.mu.Unlock()

	keyID := mounts.HashKey(key)

	_, ok := vs.snapshots[keyID]

	if !ok {
		return errors.Wrapf(errdefs.ErrNotFound, "snapshot %v", key)
	}
	params := storage.NewRemoveImageParamsWithContext(ctx).WithStoreName(vs.storageName).WithImageID(keyID)

	if _, err := vs.plClient.Storage.RemoveImage(params); err != nil {
		return err
	}

	delete(vs.snapshots, keyID)
	return nil
}

func (vs *VicSnap) Walk(ctx context.Context, fn func(context.Context, snapshot.Info) error) error {
	for _, v := range vs.snapshots {
		if v.Kind == snapshot.KindCommitted {
			fn(ctx, v)
		}
	}
	return nil
}
