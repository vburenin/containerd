package storage

import (
	"context"
	"encoding/binary"
	"fmt"

	"github.com/boltdb/bolt"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/snapshot"
	db "github.com/containerd/containerd/snapshot/storage/proto"
	"github.com/gogo/protobuf/proto"
	"github.com/pkg/errors"
)

var (
	bucketKeyStorageVersion = []byte("v1")
	bucketKeySnapshot       = []byte("snapshots")
	bucketKeyParents        = []byte("parents")

	// ErrNoTransaction is returned when an operation is attempted with
	// a context which is not inside of a transaction.
	ErrNoTransaction = errors.New("no transaction in context")
)

type boltFileTransactor struct {
	db *bolt.DB
	tx *bolt.Tx
}

func (bft *boltFileTransactor) Rollback() error {
	defer bft.db.Close()
	return bft.tx.Rollback()
}

func (bft *boltFileTransactor) Commit() error {
	defer bft.db.Close()
	return bft.tx.Commit()
}

// parentKey returns a composite key of the parent and child identifiers. The
// parts of the key are separated by a zero byte.
func parentKey(parent, child uint64) []byte {
	b := make([]byte, binary.Size([]uint64{parent, child})+1)
	i := binary.PutUvarint(b, parent)
	j := binary.PutUvarint(b[i+1:], child)
	return b[0 : i+j+1]
}

// parentPrefixKey returns the parent part of the composite key with the
// zero byte separator.
func parentPrefixKey(parent uint64) []byte {
	b := make([]byte, binary.Size(parent)+1)
	i := binary.PutUvarint(b, parent)
	return b[0 : i+1]
}

// getParentPrefix returns the first part of the composite key which
// represents the parent identifier.
func getParentPrefix(b []byte) uint64 {
	parent, _ := binary.Uvarint(b)
	return parent
}

// GetInfo returns the snapshot Info directly from the metadata. Requires a
// context with a storage transaction.
func GetInfo(ctx context.Context, key string) (string, snapshot.Info, snapshot.Usage, error) {
	var ss db.Snapshot
	err := withBucket(ctx, func(ctx context.Context, bkt, pbkt *bolt.Bucket) error {
		return getSnapshot(bkt, key, &ss)
	})
	if err != nil {
		return "", snapshot.Info{}, snapshot.Usage{}, err
	}

	usage := snapshot.Usage{
		Inodes: ss.Inodes,
		Size:   ss.Size_,
	}

	return fmt.Sprint(ss.ID), snapshot.Info{
		Name:   key,
		Parent: ss.Parent,
		Kind:   snapshot.Kind(ss.Kind),
	}, usage, nil
}

// WalkInfo iterates through all metadata Info for the stored snapshots and
// calls the provided function for each. Requires a context with a storage
// transaction.
func WalkInfo(ctx context.Context, fn func(context.Context, snapshot.Info) error) error {
	return withBucket(ctx, func(ctx context.Context, bkt, pbkt *bolt.Bucket) error {
		return bkt.ForEach(func(k, v []byte) error {
			// skip nested buckets
			if v == nil {
				return nil
			}
			var ss db.Snapshot
			if err := proto.Unmarshal(v, &ss); err != nil {
				return errors.Wrap(err, "failed to unmarshal snapshot")
			}

			info := snapshot.Info{
				Name:   string(k),
				Parent: ss.Parent,
				Kind:   snapshot.Kind(ss.Kind),
			}
			return fn(ctx, info)
		})
	})
}

// GetSnapshot returns the metadata for the active or view snapshot transaction
// referenced by the given key. Requires a context with a storage transaction.
func GetSnapshot(ctx context.Context, key string) (s Snapshot, err error) {
	err = withBucket(ctx, func(ctx context.Context, bkt, pbkt *bolt.Bucket) error {
		b := bkt.Get([]byte(key))
		if len(b) == 0 {
			return errors.Wrapf(errdefs.ErrNotFound, "snapshot %v", key)
		}

		var ss db.Snapshot
		if err := proto.Unmarshal(b, &ss); err != nil {
			return errors.Wrap(err, "failed to unmarshal snapshot")
		}

		if ss.Kind != db.KindActive && ss.Kind != db.KindView {
			return errors.Wrapf(errdefs.ErrFailedPrecondition, "requested snapshot %v not active or view", key)
		}

		s.ID = fmt.Sprintf("%d", ss.ID)
		s.Kind = snapshot.Kind(ss.Kind)

		if ss.Parent != "" {
			var parent db.Snapshot
			if err := getSnapshot(bkt, ss.Parent, &parent); err != nil {
				return errors.Wrap(err, "failed to get parent snapshot")
			}

			s.ParentIDs, err = parents(bkt, &parent)
			if err != nil {
				return errors.Wrap(err, "failed to get parent chain")
			}
		}
		return nil
	})
	if err != nil {
		return Snapshot{}, err
	}

	return
}

// CreateSnapshot inserts a record for an active or view snapshot with the provided parent.
func CreateSnapshot(ctx context.Context, kind snapshot.Kind, key, parent string) (s Snapshot, err error) {
	switch kind {
	case snapshot.KindActive, snapshot.KindView:
	default:
		return Snapshot{}, errors.Wrapf(errdefs.ErrInvalidArgument, "snapshot type %v invalid; only snapshots of type Active or View can be created", kind)
	}

	err = createBucketIfNotExists(ctx, func(ctx context.Context, bkt, pbkt *bolt.Bucket) error {
		var (
			parentS *db.Snapshot
		)
		if parent != "" {
			parentS = new(db.Snapshot)
			if err := getSnapshot(bkt, parent, parentS); err != nil {
				return errors.Wrap(err, "failed to get parent snapshot")
			}

			if parentS.Kind != db.KindCommitted {
				return errors.Wrap(errdefs.ErrInvalidArgument, "parent is not committed snapshot")
			}
		}
		b := bkt.Get([]byte(key))
		if len(b) != 0 {
			return errors.Wrapf(errdefs.ErrAlreadyExists, "snapshot %v", key)
		}

		id, err := bkt.NextSequence()
		if err != nil {
			return errors.Wrap(err, "unable to get identifier")
		}

		ss := db.Snapshot{
			ID:     id,
			Parent: parent,
			Kind:   db.Kind(kind),
		}
		if err := putSnapshot(bkt, key, &ss); err != nil {
			return err
		}

		if parentS != nil {
			// Store a backlink from the key to the parent. Store the snapshot name
			// as the value to allow following the backlink to the snapshot value.
			if err := pbkt.Put(parentKey(parentS.ID, ss.ID), []byte(key)); err != nil {
				return errors.Wrap(err, "failed to write parent link")
			}

			s.ParentIDs, err = parents(bkt, parentS)
			if err != nil {
				return errors.Wrap(err, "failed to get parent chain")
			}
		}

		s.ID = fmt.Sprintf("%d", id)
		s.Kind = kind
		return nil
	})
	if err != nil {
		return Snapshot{}, err
	}

	return
}

// Remove removes a snapshot from the metastore. The string identifier for the
// snapshot is returned as well as the kind. The provided context must contain a
// writable transaction.
func Remove(ctx context.Context, key string) (id string, k snapshot.Kind, err error) {
	err = withBucket(ctx, func(ctx context.Context, bkt, pbkt *bolt.Bucket) error {
		var ss db.Snapshot
		b := bkt.Get([]byte(key))
		if len(b) == 0 {
			return errors.Wrapf(errdefs.ErrNotFound, "snapshot %v", key)
		}

		if err := proto.Unmarshal(b, &ss); err != nil {
			return errors.Wrap(err, "failed to unmarshal snapshot")
		}

		if pbkt != nil {
			k, _ := pbkt.Cursor().Seek(parentPrefixKey(ss.ID))
			if getParentPrefix(k) == ss.ID {
				return errors.Errorf("cannot remove snapshot with child")
			}

			if ss.Parent != "" {
				var ps db.Snapshot
				if err := getSnapshot(bkt, ss.Parent, &ps); err != nil {
					return errors.Wrap(err, "failed to get parent snapshot")
				}

				if err := pbkt.Delete(parentKey(ps.ID, ss.ID)); err != nil {
					return errors.Wrap(err, "failed to delte parent link")
				}
			}
		}

		if err := bkt.Delete([]byte(key)); err != nil {
			return errors.Wrap(err, "failed to delete snapshot")
		}

		id = fmt.Sprintf("%d", ss.ID)
		k = snapshot.Kind(ss.Kind)

		return nil
	})

	return
}

// CommitActive renames the active snapshot transaction referenced by `key`
// as a committed snapshot referenced by `Name`. The resulting snapshot  will be
// committed and readonly. The `key` reference will no longer be available for
// lookup or removal. The returned string identifier for the committed snapshot
// is the same identifier of the original active snapshot. The provided context
// must contain a writable transaction.
func CommitActive(ctx context.Context, key, name string, usage snapshot.Usage) (id string, err error) {
	err = withBucket(ctx, func(ctx context.Context, bkt, pbkt *bolt.Bucket) error {
		b := bkt.Get([]byte(name))
		if len(b) != 0 {
			return errors.Wrapf(errdefs.ErrAlreadyExists, "committed snapshot %v", name)
		}

		var ss db.Snapshot
		if err := getSnapshot(bkt, key, &ss); err != nil {
			return errors.Wrap(err, "failed to get active snapshot")
		}
		if ss.Kind != db.KindActive {
			return errors.Wrapf(errdefs.ErrFailedPrecondition, "snapshot %v is not active", name)
		}

		ss.Kind = db.KindCommitted
		ss.Inodes = usage.Inodes
		ss.Size_ = usage.Size

		if err := putSnapshot(bkt, name, &ss); err != nil {
			return err
		}
		if err := bkt.Delete([]byte(key)); err != nil {
			return errors.Wrap(err, "failed to delete active")
		}
		if ss.Parent != "" {
			var ps db.Snapshot
			if err := getSnapshot(bkt, ss.Parent, &ps); err != nil {
				return errors.Wrap(err, "failed to get parent snapshot")
			}

			// Updates parent back link to use new key
			if err := pbkt.Put(parentKey(ps.ID, ss.ID), []byte(name)); err != nil {
				return errors.Wrap(err, "failed to update parent link")
			}
		}

		id = fmt.Sprintf("%d", ss.ID)

		return nil
	})
	if err != nil {
		return "", err
	}

	return
}

func withBucket(ctx context.Context, fn func(context.Context, *bolt.Bucket, *bolt.Bucket) error) error {
	t, ok := ctx.Value(transactionKey{}).(*boltFileTransactor)
	if !ok {
		return ErrNoTransaction
	}
	bkt := t.tx.Bucket(bucketKeyStorageVersion)
	if bkt == nil {
		return errors.Wrap(errdefs.ErrNotFound, "bucket does not exist")
	}
	return fn(ctx, bkt.Bucket(bucketKeySnapshot), bkt.Bucket(bucketKeyParents))
}

func createBucketIfNotExists(ctx context.Context, fn func(context.Context, *bolt.Bucket, *bolt.Bucket) error) error {
	t, ok := ctx.Value(transactionKey{}).(*boltFileTransactor)
	if !ok {
		return ErrNoTransaction
	}

	bkt, err := t.tx.CreateBucketIfNotExists(bucketKeyStorageVersion)
	if err != nil {
		return errors.Wrap(err, "failed to create version bucket")
	}
	sbkt, err := bkt.CreateBucketIfNotExists(bucketKeySnapshot)
	if err != nil {
		return errors.Wrap(err, "failed to create snapshots bucket")
	}
	pbkt, err := bkt.CreateBucketIfNotExists(bucketKeyParents)
	if err != nil {
		return errors.Wrap(err, "failed to create snapshots bucket")
	}
	return fn(ctx, sbkt, pbkt)
}

func parents(bkt *bolt.Bucket, parent *db.Snapshot) (parents []string, err error) {
	for {
		parents = append(parents, fmt.Sprintf("%d", parent.ID))

		if parent.Parent == "" {
			return
		}

		var ps db.Snapshot
		if err := getSnapshot(bkt, parent.Parent, &ps); err != nil {
			return nil, errors.Wrap(err, "failed to get parent snapshot")
		}
		parent = &ps
	}
}

func getSnapshot(bkt *bolt.Bucket, key string, ss *db.Snapshot) error {
	b := bkt.Get([]byte(key))
	if len(b) == 0 {
		return errors.Wrapf(errdefs.ErrNotFound, "snapshot %v", key)
	}
	if err := proto.Unmarshal(b, ss); err != nil {
		return errors.Wrap(err, "failed to unmarshal snapshot")
	}
	return nil
}

func putSnapshot(bkt *bolt.Bucket, key string, ss *db.Snapshot) error {
	b, err := proto.Marshal(ss)
	if err != nil {
		return errors.Wrap(err, "failed to marshal snapshot")
	}

	if err := bkt.Put([]byte(key), b); err != nil {
		return errors.Wrap(err, "failed to save snapshot")
	}
	return nil
}
