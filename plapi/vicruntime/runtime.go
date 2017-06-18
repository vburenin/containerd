package vicruntime

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
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

var (
	ErrTaskNotExists     = errors.New("task does not exist")
	ErrTaskAlreadyExists = errors.New("task already exists")
	pluginID             = fmt.Sprintf("%s.%s", plugin.RuntimePlugin, "vmware-linux")
)

func init() {
	plugin.Register(&plugin.Registration{
		ID:     "vmware-linux",
		Type:   plugin.RuntimePlugin,
		Init:   New,
		Config: vicconfig.DefaultConfig(),
		Requires: []plugin.PluginType{
			plugin.TaskMonitorPlugin,
		},
	})
}

type Runtime struct {
	root string
	mu   sync.Mutex

	events        chan *plugin.Event
	pl            *client.PortLayer
	eventsContext context.Context
	eventsCancel  func()
	tasks         map[string]plugin.Task
	monitor       plugin.TaskMonitor
}

var _ plugin.Runtime = &Runtime{}

func New(ic *plugin.InitContext) (interface{}, error) {
	if err := os.MkdirAll(ic.Root, 0700); err != nil {
		return nil, err
	}

	monitor, err := ic.Get(plugin.TaskMonitorPlugin)
	if err != nil {
		return nil, err
	}

	cfg := ic.Config.(*vicconfig.Config)
	logrus.Infof("Vic runtime config: %q", cfg)

	c, cancel := context.WithCancel(ic.Context)
	r := &Runtime{
		root:          ic.Root,
		events:        make(chan *plugin.Event, 2048),
		eventsContext: c,
		eventsCancel:  cancel,
		pl:            PortLayerClient(cfg.PortlayerAddress),
		monitor:       monitor.(plugin.TaskMonitor),
	}

	if err := r.updateContainerList(ic.Context); err != nil {
		return nil, err
	}

	r.monitor.Events(r.events)

	return r, nil
}

func (r *Runtime) updateContainerList(ctx context.Context) error {
	logrus.Debugf("Refreshing running tasks list")

	r.mu.Lock()
	defer r.mu.Unlock()

	params := containers.NewGetContainerListParamsWithContext(ctx).WithAll(swag.Bool(true))

	cl, err := r.pl.Containers.GetContainerList(params)
	if err != nil {
		return err
	}
	logrus.Debugf("Discovered %d tasks", len(cl.Payload))
	r.tasks = make(map[string]plugin.Task, len(cl.Payload))
	for _, c := range cl.Payload {
		r.tasks[c.ContainerConfig.ContainerID] = RestoreVicContaner(ctx, r.pl, r.root, c)
	}
	return nil
}

func (r *Runtime) ID() string {
	return pluginID
}

func (r *Runtime) Create(ctx context.Context, id string, opts plugin.CreateOpts) (plugin.Task, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.tasks[id]; ok {
		return nil, ErrTaskAlreadyExists
	}

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

func (r *Runtime) Get(ctx context.Context, id string) (plugin.Task, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.tasks[id]
	if !ok {
		return nil, ErrTaskNotExists
	}
	return t, nil
}

func (r *Runtime) Tasks(ctx context.Context) ([]plugin.Task, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	o := make([]plugin.Task, 0, len(r.tasks))
	for _, t := range r.tasks {
		o = append(o, t)
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
