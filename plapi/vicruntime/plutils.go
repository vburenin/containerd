package vicruntime

import (
	"context"
	"fmt"

	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/plapi/client"
	"github.com/containerd/containerd/plapi/client/containers"
	"github.com/go-openapi/swag"
)

func GetHandle(ctx context.Context, pl *client.PortLayer, id string) (string, error) {
	r, err := pl.Containers.Get(containers.NewGetParamsWithContext(ctx).WithID(id))
	if err != nil {
		log.G(ctx).Warningf("Could not get handle for container: %s", err)
		return "", fmt.Errorf("Could not get container handle: %s", id)
	}

	return r.Payload, nil
}

func CommitHandle(ctx context.Context, pl *client.PortLayer, h string) error {
	log.G(ctx).Debugf("Committing handle: %s", h)
	commitParams := containers.NewCommitParamsWithContext(ctx).WithHandle(h).WithWaitTime(swag.Int32(5))
	_, err := pl.Containers.Commit(commitParams)
	if err != nil {
		log.G(ctx).Warningf("Could not commit handle %s: %s", h, err)
	}
	return err
}
