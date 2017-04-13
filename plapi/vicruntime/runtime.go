package vicruntime

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/api/services/shim"
	"github.com/containerd/containerd/api/types/container"
	"github.com/containerd/containerd/api/types/mount"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/plapi/client"
	"github.com/containerd/containerd/plapi/client/containers"
	"github.com/containerd/containerd/plugin"
	"github.com/go-openapi/swag"
	"golang.org/x/net/context"
)

const (
	runtimeName    = "vic-linux"
	configFilename = "config.json"
)

func init() {
	plugin.Register(runtimeName, &plugin.Registration{
		Type: plugin.RuntimePlugin,
		Init: New,
	})
}

type Runtime struct {
	root string

	events        chan *containerd.Event
	client        *client.PortLayer
	eventsContext context.Context
	eventsCancel  func()
}

func New(ic *plugin.InitContext) (interface{}, error) {
	path := filepath.Join(ic.State, runtimeName)
	if err := os.MkdirAll(path, 0700); err != nil {
		return nil, err
	}
	c, cancel := context.WithCancel(ic.Context)
	rt := &Runtime{
		root:          path,
		events:        make(chan *containerd.Event, 2048),
		eventsContext: c,
		eventsCancel:  cancel,
		client:        PortLayerClient(),
	}

	return rt, nil
}

func (r *Runtime) Create(ctx context.Context, id string, opts containerd.CreateOpts) (containerd.Container, error) {
	logrus.Debugf("Starting runtime for %s. Options: %q", id, opts)
	path, err := r.newBundle(id, opts.Spec)
	if err != nil {
		return nil, err
	}

	shimSock := filepath.Join(path, "shim.sock")
	s, err := NewPortLayerShim(r.root, shimSock, nil)
	if err != nil {
		os.RemoveAll(path)
		return nil, err
	}
	if err := r.handleEvents(s); err != nil {
		os.RemoveAll(path)
		return nil, err
	}
	sopts := &shim.CreateRequest{
		ID:       id,
		Bundle:   path,
		Runtime:  "runc",
		Stdin:    opts.IO.Stdin,
		Stdout:   opts.IO.Stdout,
		Stderr:   opts.IO.Stderr,
		Terminal: opts.IO.Terminal,
	}
	for _, m := range opts.Rootfs {
		sopts.Rootfs = append(sopts.Rootfs, &mount.Mount{
			Type:    m.Type,
			Source:  m.Source,
			Options: m.Options,
		})
	}
	if _, err := s.Create(ctx, sopts); err != nil {
		os.RemoveAll(path)
		return nil, err
	}
	return &Container{
		id:   id,
		shim: s,
	}, nil
}

func (r *Runtime) Delete(ctx context.Context, c containerd.Container) (uint32, error) {
	lc, ok := c.(*Container)
	if !ok {
		return 0, fmt.Errorf("container cannot be cast as *linux.Container")
	}
	if _, err := lc.shim.Delete(ctx, &shim.DeleteRequest{}); err != nil {
		return 0, err
	}
	lc.shim.Exit(ctx, &shim.ExitRequest{})
	return 0, r.deleteBundle(lc.id)
}

func (r *Runtime) Containers(ctx context.Context) ([]containerd.Container, error) {

	var o []containerd.Container

	pl := PortLayerClient()
	params := containers.NewGetContainerListParams().WithAll(swag.Bool(true))

	cl, err := pl.Containers.GetContainerList(params)
	if err != nil {
		return nil, err
	}

	for _, c := range cl.Payload {
		shimSock := filepath.Join(r.root, c.ContainerConfig.ContainerID, "shim.sock")
		shimClient, err := NewPortLayerShim(r.root, shimSock, c)
		if err != nil {
			logrus.WithError(err).Warningf(
				"Failed to initialize existing container %s",
				c.ContainerConfig.ContainerID)
		} else {

			// Portlayer ID doesn't match containerd ID.
			cnt := &Container{
				id:   c.ContainerConfig.Annotations[AnnotationContainerdID],
				shim: shimClient,
			}
			// If container is not created by containerd use Portlayer ID.
			if cnt.id == "" {
				cnt.id = c.ContainerConfig.ContainerID
			}
			logrus.Debugf("Restoring shim state for containerd ID %s (PL ID: %s)",
				c.ContainerConfig.Annotations[AnnotationContainerdID],
				c.ContainerConfig.ContainerID)
			o = append(o, cnt)
		}
	}

	return o, nil
}

func (r *Runtime) Events(ctx context.Context) <-chan *containerd.Event {
	return r.events
}

func (r *Runtime) handleEvents(s shim.ShimClient) error {
	events, err := s.Events(r.eventsContext, &shim.EventsRequest{})
	if err != nil {
		return err
	}
	go r.forward(events)
	return nil
}

func (r *Runtime) forward(events shim.Shim_EventsClient) {
	for {
		e, err := events.Recv()
		if err != nil {
			log.G(r.eventsContext).WithError(err).Error("get event from shim")
			return
		}
		var et containerd.EventType
		switch e.Type {
		case container.Event_CREATE:
			et = containerd.CreateEvent
		case container.Event_EXEC_ADDED:
			et = containerd.ExecAddEvent
		case container.Event_EXIT:
			et = containerd.ExitEvent
		case container.Event_OOM:
			et = containerd.OOMEvent
		case container.Event_START:
			et = containerd.StartEvent
		}
		r.events <- &containerd.Event{
			Timestamp:  time.Now(),
			Runtime:    runtimeName,
			Type:       et,
			Pid:        e.Pid,
			ID:         e.ID,
			ExitStatus: e.ExitStatus,
		}
	}
}

func (r *Runtime) newBundle(id string, spec []byte) (string, error) {
	path := filepath.Join(r.root, id)
	if err := os.Mkdir(path, 0700); err != nil {
		return "", err
	}
	if err := os.Mkdir(filepath.Join(path, "rootfs"), 0700); err != nil {
		return "", err
	}
	f, err := os.Create(filepath.Join(path, configFilename))
	if err != nil {
		return "", err
	}
	_, err = io.Copy(f, bytes.NewReader(spec))
	return path, err
}

func (r *Runtime) deleteBundle(id string) error {
	return os.RemoveAll(filepath.Join(r.root, id))
}
