package vicruntime

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/containerd/containerd/api/services/shim"
	"github.com/containerd/containerd/api/types/task"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/plapi/client"
	"github.com/containerd/containerd/plapi/client/containers"
	"github.com/containerd/containerd/plapi/vicconfig"
	"github.com/containerd/containerd/plugin"
	"github.com/go-openapi/swag"
)

const (
	runtimeName    = "vic"
	configFilename = "config.json"
)

func init() {
	plugin.Register(runtimeName, &plugin.Registration{
		Type:   plugin.RuntimePlugin,
		Init:   New,
		Config: vicconfig.DefaultConfig(),
	})
}

type Runtime struct {
	root string

	events        chan *plugin.Event
	pl            *client.PortLayer
	eventsContext context.Context
	eventsCancel  func()
}

var _ plugin.Runtime = &Runtime{}

func New(ic *plugin.InitContext) (interface{}, error) {
	path := filepath.Join(ic.State, runtimeName)
	if err := os.MkdirAll(path, 0700); err != nil {
		return nil, err
	}

	cfg := ic.Config.(*vicconfig.Config)
	logrus.Infof("Vic runtime config: %q", cfg)

	c, cancel := context.WithCancel(ic.Context)
	r := &Runtime{
		root:          path,
		events:        make(chan *plugin.Event, 2048),
		eventsContext: c,
		eventsCancel:  cancel,
		pl:            PortLayerClient(cfg.PortlayerAddress),
	}
	ic.Monitor.Events(r.events)

	return r, nil
}

func (r *Runtime) Create(ctx context.Context, id string, opts plugin.CreateOpts) (plugin.Task, error) {
	logrus.Debugf("Starting runtime for %s. Options: %q", id, opts)
	path, err := r.newBundle(id, opts.Spec)
	if err != nil {
		return nil, err
	}

	s, err := NewVicTask(ctx, r.pl, r.root, id, opts)
	if err != nil {
		os.RemoveAll(path)
		return nil, err
	}

	return s, err
}

func (r *Runtime) Tasks(ctx context.Context) ([]plugin.Task, error) {

	var o []plugin.Task

	params := containers.NewGetContainerListParams().WithAll(swag.Bool(true))

	cl, err := r.pl.Containers.GetContainerList(params)
	if err != nil {
		return nil, err
	}

	for _, c := range cl.Payload {
		o = append(o, RestoreVicContaner(ctx, r.pl, r.root, c))
	}

	return o, nil
}

func (r *Runtime) Delete(ctx context.Context, c plugin.Task) (*plugin.Exit, error) {
	return &plugin.Exit{
		Status:    0,
		Timestamp: time.Now(),
	}, nil
}

func (r *Runtime) Events(ctx context.Context) <-chan *plugin.Event {
	return r.events
}

func (r *Runtime) forward(events shim.Shim_EventsClient) {
	for {
		e, err := events.Recv()
		if err != nil {
			log.G(r.eventsContext).WithError(err).Error("get event from shim")
			return
		}
		var et plugin.EventType
		switch e.Type {
		case task.Event_CREATE:
			et = plugin.CreateEvent
		case task.Event_EXEC_ADDED:
			et = plugin.ExecAddEvent
		case task.Event_EXIT:
			et = plugin.ExitEvent
		case task.Event_OOM:
			et = plugin.OOMEvent
		case task.Event_START:
			et = plugin.StartEvent
		}
		r.events <- &plugin.Event{
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
