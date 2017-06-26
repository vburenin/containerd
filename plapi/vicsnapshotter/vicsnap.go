package vicsnapshotter

import (
	"context"
	"fmt"
	"sync"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/mount"
	"github.com/containerd/containerd/plapi/client"
	"github.com/containerd/containerd/plapi/client/storage"
	"github.com/containerd/containerd/plapi/models"
	"github.com/containerd/containerd/plapi/vicconfig"
	"github.com/containerd/containerd/plapi/vicruntime"
	"github.com/containerd/containerd/plugin"
	"github.com/containerd/containerd/snapshot"
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
		vs.snapshots[img.ID] = snapshot.Info{
			Parent:   img.Metadata["parent"],
			Name:     img.ID,
			Readonly: false,
			Kind:     snapshot.KindCommitted,
		}
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
		return snapshot.Info{}, snapshot.ErrSnapshotNotExist
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
	si, ok := vs.snapshots[key]
	if !ok {
		return nil, fmt.Errorf("Key %s not found", key)
	}
	if si.Kind != snapshot.KindActive {
		return nil, fmt.Errorf("Snapshot %s is not active", key)
	}
	mounts, ok := vs.active[key]
	if !ok {
		log.G(ctx).WithField("key", key).Errorf("No mounts found for the active snapshot")
		return nil, fmt.Errorf("No mounts found for the active snapshot %s", key)
	}
	return mounts, nil
}

func (vs *VicSnap) addActive(snapType, key, parent string) []mount.Mount {
	m := mount.Mount{
		Options: []string{"rw"},
		Type:    "ext4",
		Source:  fmt.Sprintf("%s/%s/%s", snapType, parent, key),
	}

	vs.snapshots[key] = snapshot.Info{
		Name:     key,
		Parent:   parent,
		Kind:     snapshot.KindActive,
		Readonly: snapType == "view",
	}

	mountList := []mount.Mount{m}
	vs.active[key] = mountList
	return mountList
}

func (vs *VicSnap) prepareSnapshot(ctx context.Context, snapType, key, parent string) ([]mount.Mount, error) {
	log.G(ctx).Debugf("Preparing snapshot %s:%s/%s", snapType, parent, key)
	vs.mu.Lock()
	defer vs.mu.Unlock()

	if _, ok := vs.snapshots[key]; ok {
		return nil, snapshot.ErrSnapshotExist
	}

	if parent != "" {
		if p, ok := vs.snapshots[parent]; !ok {
			return nil, snapshot.ErrSnapshotNotExist
		} else {
			if p.Kind == snapshot.KindActive {
				return nil, snapshot.ErrSnapshotNotCommitted
			}
		}
	} else {
		parent = "scratch"
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
	s, ok := vs.snapshots[key]
	if !ok {
		return snapshot.ErrSnapshotNotExist
	}
	if s.Kind != snapshot.KindActive {
		return snapshot.ErrSnapshotNotActive
	}

	if _, ok := vs.snapshots[name]; ok {
		return snapshot.ErrSnapshotExist
	}

	params := storage.NewCommitImageParamsWithContext(ctx).
		WithStoreName(vs.storageName).
		WithNewID(name).
		WithOldID(key)

	_, err := vs.plClient.Storage.CommitImage(params)
	if err == nil {
		vs.snapshots[name] = snapshot.Info{
			Name:     name,
			Parent:   s.Parent,
			Kind:     snapshot.KindCommitted,
			Readonly: true,
		}
	}
	return err
}

func (vs *VicSnap) Remove(ctx context.Context, key string) error {
	vs.mu.Lock()
	defer vs.mu.Unlock()

	s, ok := vs.snapshots[key]

	if !ok {
		return snapshot.ErrSnapshotNotExist
	}
	if s.Kind != snapshot.KindActive {
		return snapshot.ErrSnapshotNotActive
	}
	params := storage.NewRemoveImageParamsWithContext(ctx).WithStoreName(vs.storageName).WithImageID(key)

	if _, err := vs.plClient.Storage.RemoveImage(params); err != nil {
		return err
	}

	delete(vs.snapshots, key)
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
