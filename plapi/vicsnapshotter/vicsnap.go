package vicsnapshotter

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"

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

const PortLayerContainerMetadataKey = "ctd_data"

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

	// hash2Key is used to check existence of the snapshot based on its key.
	hash2Key map[string]string
	store    content.Store

	mu sync.Mutex
}

func dumpSnapInfo(info *snapshot.Info) (string, error) {
	data, err := json.Marshal(info)
	return string(data), err
}

func loadSnapInfo(data string) (*snapshot.Info, error) {
	si := &snapshot.Info{}
	err := json.Unmarshal([]byte(data), si)
	return si, err
}

func NewVicSnap(ic *plugin.InitContext) (interface{}, error) {
	cfg := ic.Config.(*vicconfig.Config)
	vs := &VicSnap{
		storageName: "containerd-storage",
		plClient:    vicruntime.PortLayerClient(cfg.PortlayerAddress),
		snapshots:   make(map[string]snapshot.Info),
		active:      make(map[string][]mount.Mount),
		hash2Key:    make(map[string]string),
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

	l := log.G(ctx)
	l.Infof("Found %d images", len(resp.Payload))
	for _, img := range resp.Payload {
		snapInfo, err := loadSnapInfo(img.Metadata[PortLayerContainerMetadataKey])
		if err != nil {
			l.WithError(err).Error("Could not parse image info")
			continue
		}

		if snapInfo.Kind == snapshot.KindActive {
			vs.addActive("img", snapInfo)
		}

		vs.snapshots[snapInfo.Name] = *snapInfo
		vs.hash2Key[mounts.HashKey(snapInfo.Name)] = snapInfo.Name

		l.Debugf("Discovered image %#v", img)
	}
	return nil
}

func (vs *VicSnap) StatByHash(ctx context.Context, keyHash string) (snapshot.Info, error) {
	vs.mu.Lock()
	defer vs.mu.Unlock()
	key, ok := vs.hash2Key[keyHash]

	if !ok {
		return snapshot.Info{}, errors.Wrapf(errdefs.ErrNotFound, "no snapshot with hash %q", key)
	}

	si, ok := vs.snapshots[key]

	if !ok {
		return snapshot.Info{}, errors.Wrapf(errdefs.ErrNotFound, "snapshot %v", key)
	}

	return si, nil
}

func (vs *VicSnap) Stat(ctx context.Context, key string) (snapshot.Info, error) {
	vs.mu.Lock()
	defer vs.mu.Unlock()
	si, ok := vs.snapshots[key]

	if !ok {
		return snapshot.Info{}, errors.Wrapf(errdefs.ErrNotFound, "snapshot %v", key)
	}
	return si, nil
}

func (vs *VicSnap) Update(ctx context.Context, info snapshot.Info, fieldpaths ...string) (snapshot.Info, error) {
	vs.mu.Lock()
	defer vs.mu.Unlock()

	keyID := mounts.HashKey(info.Name)
	si, ok := vs.snapshots[info.Name]

	if !ok {
		return snapshot.Info{}, errors.Wrapf(errdefs.ErrNotFound, "snapshot %v", info.Name)
	}

	updated := snapshot.Info{
		Name:    info.Name,
		Kind:    si.Kind,
		Parent:  si.Parent,
		Created: si.Created,
	}

	err := func() error {
		if len(fieldpaths) > 0 {
			for _, path := range fieldpaths {
				if strings.HasPrefix(path, "labels.") {
					if updated.Labels == nil {
						updated.Labels = map[string]string{}
					}

					key := strings.TrimPrefix(path, "labels.")
					updated.Labels[key] = info.Labels[key]
					continue
				}

				switch path {
				case "labels":
					updated.Labels = info.Labels
				default:
					return errors.Wrapf(errdefs.ErrInvalidArgument, "cannot update %q field on snapshot %q", path, info.Name)
				}
			}
		} else {
			// Set mutable fields
			updated.Labels = info.Labels
		}

		updated.Updated = time.Now().UTC()

		data, err := dumpSnapInfo(&updated)
		if err != nil {
			return errors.Wrapf(errdefs.ErrUnknown, "failed to store metadata: %s", err)
		}
		params := &storage.UpdateImageMetadataParams{
			Context:   ctx,
			Meta:      []string{PortLayerContainerMetadataKey, data},
			StoreName: vs.storageName,
			ImageID:   keyID,
		}
		_, err = vs.plClient.Storage.UpdateImageMetadata(params)
		if err != nil {
			return errors.Wrapf(errdefs.ErrUnknown, "failed to store metadata: %s", err)
		}
		return nil
	}()

	if err != nil {
		return snapshot.Info{}, err
	}

	return updated, nil
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
		return nil, errors.Wrapf(errdefs.ErrNotFound, "snapshot %v", key)
	}
	if si.Kind != snapshot.KindActive {
		return nil, errors.Wrapf(errdefs.ErrFailedPrecondition, "snapshot %v", key)
	}
	snapMounts, ok := vs.active[key]
	if !ok {
		log.G(ctx).WithField("key", key).Errorf("No mounts found for the active snapshot")
		return nil, errors.Wrapf(errdefs.ErrNotFound, "active snapshot %v", key)
	}
	return snapMounts, nil
}

func (vs *VicSnap) addActive(snapType string, si *snapshot.Info) []mount.Mount {
	m := mount.Mount{
		Options: []string{"rw"},
		Type:    "ext4",
		Source:  mounts.FormatMountSource(snapType, si),
	}

	mountList := []mount.Mount{m}
	vs.active[si.Name] = mountList
	return mountList
}

func (vs *VicSnap) prepareSnapshot(ctx context.Context, snapType, key, parent string,
	opts []snapshot.Opt) ([]mount.Mount, error) {
	log.G(ctx).Debugf("Preparing snapshot %s:%s/%s", snapType, parent, key)

	vs.mu.Lock()
	defer vs.mu.Unlock()

	if _, ok := vs.snapshots[key]; ok {
		return nil, errors.Wrapf(errdefs.ErrAlreadyExists, "snapshot %v", key)
	}

	if parent == "" {
		parent = "scratch"
	}

	if parent != "scratch" {
		if p, ok := vs.snapshots[parent]; !ok {
			return nil, errors.Wrapf(errdefs.ErrNotFound, "snapshot %v", key)
		} else {
			if p.Kind == snapshot.KindActive {
				return nil, errors.Wrapf(errdefs.ErrInvalidArgument, "snapshot %v", key)
			}
		}
	}

	si := &snapshot.Info{
		Parent:  parent,
		Name:    key,
		Kind:    snapshot.KindActive,
		Created: time.Now().UTC(),
		Updated: time.Now().UTC(),
	}

	for _, opt := range opts {
		opt(si)
	}

	vs.snapshots[key] = *si
	vs.hash2Key[mounts.HashKey(si.Name)] = si.Name
	return vs.addActive(snapType, si), nil
}

func (vs *VicSnap) Prepare(ctx context.Context, key, parent string, opts ...snapshot.Opt) ([]mount.Mount, error) {
	return vs.prepareSnapshot(ctx, "img", key, parent, opts)
}

func (vs *VicSnap) View(ctx context.Context, key, parent string, opts ...snapshot.Opt) ([]mount.Mount, error) {
	return vs.prepareSnapshot(ctx, "view", key, parent, opts)
}

func (vs *VicSnap) Commit(ctx context.Context, name, key string, opts ...snapshot.Opt) error {
	vs.mu.Lock()
	defer vs.mu.Unlock()

	s, ok := vs.snapshots[key]
	if !ok {
		return errors.Wrapf(errdefs.ErrNotFound, "snapshot %v", key)
	}
	if s.Kind != snapshot.KindActive {
		return errors.Wrapf(errdefs.ErrFailedPrecondition, "requested snapshot %v not active", key)
	}

	if _, ok := vs.snapshots[name]; ok {
		return errors.Wrapf(errdefs.ErrAlreadyExists, "snapshot %v", key)
	}

	si := &snapshot.Info{
		Name:    name,
		Parent:  s.Parent,
		Kind:    snapshot.KindCommitted,
		Created: time.Now().UTC(),
		Updated: time.Now().UTC(),
		Labels:  s.Labels,
	}

	for _, opt := range opts {
		opt(si)
	}

	binSi, err := dumpSnapInfo(si)
	if err != nil {
		return errors.Wrapf(errdefs.ErrUnknown, "Serialization error: %s", err)
	}

	keyID := mounts.HashKey(key)
	nameID := mounts.HashKey(name)

	params := storage.NewCommitImageParamsWithContext(ctx).
		WithStoreName(vs.storageName).
		WithCommitID(nameID).
		WithActiveID(keyID).
		WithMeta([]string{PortLayerContainerMetadataKey, binSi})

	_, err = vs.plClient.Storage.CommitImage(params)
	if err == nil {
		vs.snapshots[name] = *si
		vs.hash2Key[mounts.HashKey(si.Name)] = si.Name
	}
	return err
}

func (vs *VicSnap) Remove(ctx context.Context, key string) error {
	vs.mu.Lock()
	defer vs.mu.Unlock()

	_, ok := vs.snapshots[key]

	if !ok {
		return errors.Wrapf(errdefs.ErrNotFound, "snapshot %v", key)
	}

	params := storage.NewRemoveImageParamsWithContext(ctx).
		WithStoreName(vs.storageName).
		WithImageID(mounts.HashKey(key))

	if _, err := vs.plClient.Storage.RemoveImage(params); err != nil {
		return err
	}

	delete(vs.snapshots, key)
	delete(vs.hash2Key, mounts.HashKey(key))
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
