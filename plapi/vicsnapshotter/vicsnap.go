package vicsnapshotter

import (
	"context"
	"fmt"
	"sync"

	"github.com/Sirupsen/logrus"
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
	plugin.Register("snapshot-vic", &plugin.Registration{
		Type: plugin.SnapshotPlugin,
		Init: func(ic *plugin.InitContext) (interface{}, error) {
			return NewVicSnap(ic), nil
		},
		Config: vicconfig.DefaultConfig(),
	})
}

type VicMount struct {
	parent   string
	fs       string
	current  string
	snapType string
	target   string
	options  []string
}

type VicSnap struct {
	storageName    string
	plClient       *client.PortLayer
	snapshots      map[string]snapshot.Info
	parentChildMap map[string][]string
	active         map[string][]mount.Mount
	mounts         map[string]*VicMount
	store          content.Store

	mu sync.Mutex
}

func NewVicSnap(ic *plugin.InitContext) *VicSnap {
	cfg := ic.Config.(*vicconfig.Config)
	vs := &VicSnap{
		storageName:    "containerd-storage",
		plClient:       vicruntime.PortLayerClient(cfg.PortlayerAddress),
		snapshots:      make(map[string]snapshot.Info),
		parentChildMap: make(map[string][]string),
		active:         make(map[string][]mount.Mount),
		mounts:         make(map[string]*VicMount),
	}
	vs.createStorage()
	return vs
}

var _ snapshot.Snapshotter = &VicSnap{}

func (vs *VicSnap) createStorage() {
	is := models.ImageStore{
		Name: vs.storageName,
	}
	r, err := vs.plClient.Storage.CreateImageStore(
		storage.NewCreateImageStoreParamsWithContext(context.TODO()).WithBody(&is),
	)
	if err != nil {
		logrus.Warnf("Storage is not created: %s", err)
		return
	}
	logrus.Debugf("Storage has been created: %s", r.Payload.URL)

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

func (vs *VicSnap) prepareSnapshot(snapType, key, parent string) ([]mount.Mount, error) {
	logrus.Debugf("Preparing snapshot %s:%s/%s", snapType, parent, key)
	vs.mu.Lock()
	defer vs.mu.Unlock()

	if _, ok := vs.snapshots[key]; ok {
		return nil, snapshot.ErrSnapshotExist
	}

	if parent != "" {
		if _, ok := vs.snapshots[parent]; !ok {
			return nil, snapshot.ErrSnapshotNotExist
		}
	} else {
		parent = "scratch"
	}

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

	return mountList, nil
}

func (vs *VicSnap) Prepare(ctx context.Context, key, parent string) ([]mount.Mount, error) {
	return vs.prepareSnapshot("img", key, parent)
}

func (vs *VicSnap) View(ctx context.Context, key, parent string) ([]mount.Mount, error) {
	return vs.prepareSnapshot("view", key, parent)
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
	vs.snapshots[name] = snapshot.Info{
		Name:     name,
		Parent:   s.Parent,
		Kind:     snapshot.KindCommitted,
		Readonly: true,
	}

	params := storage.NewCommitImageParamsWithContext(ctx).
		WithStoreName(vs.storageName).
		WithNewID(name).
		WithOldID(key)

	_, err := vs.plClient.Storage.CommitImage(params)
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
	logrus.Info("VicSnap Walk call")
	return nil
}
