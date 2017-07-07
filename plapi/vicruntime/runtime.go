package vicruntime

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	events "github.com/containerd/containerd/api/services/events/v1"
	"github.com/containerd/containerd/plapi/client"
	"github.com/containerd/containerd/plapi/vicconfig"
	"github.com/containerd/containerd/plugin"
	"github.com/containerd/containerd/runtime"
	"github.com/pkg/errors"
)

const (
	runtimeName    = "vmware-linux"
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

	events        chan *events.RuntimeEvent
	pl            *client.PortLayer
	eventsContext context.Context
	eventsCancel  func()
	tasks         map[string]runtime.Task
	monitor       runtime.TaskMonitor
}

var _ runtime.Runtime = &Runtime{}

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
		events:        make(chan *events.RuntimeEvent, 4096),
		eventsContext: c,
		eventsCancel:  cancel,
		pl:            PortLayerClient(cfg.PortlayerAddress),
		monitor:       monitor.(runtime.TaskMonitor),
		tasks:         make(map[string]runtime.Task),
	}

	if err := r.updateContainerList(ic.Context); err != nil {
		return nil, err
	}

	r.monitor.Events(r.events)

	return r, nil
}

func (r *Runtime) updateContainerList(ctx context.Context) error {
	//logrus.Debugf("Refreshing running tasks list")
	//
	//r.mu.Lock()
	//defer r.mu.Unlock()
	//
	//params := containers.NewGetContainerListParamsWithContext(ctx).WithAll(swag.Bool(true))
	//
	//cl, err := r.pl.Containers.GetContainerList(params)
	//if err != nil {
	//	return err
	//}
	//logrus.Debugf("Discovered %d tasks", len(cl.Payload))
	//r.tasks = make(map[string]plugin.Task, len(cl.Payload))
	//for _, c := range cl.Payload {
	//	r.tasks[c.ContainerConfig.ContainerID] = RestoreVicContaner(ctx, r.pl, r.root, c)
	//}
	return nil
}

func (r *Runtime) ID() string {
	return pluginID
}

func (r *Runtime) Create(ctx context.Context, id string, opts runtime.CreateOpts) (runtime.Task, error) {
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

	s := NewVicTasker(ctx, r.pl,
		r.events, "containerd-storage", path, id)

	if err := s.Create(ctx, opts); err != nil {
		os.RemoveAll(path)
		return nil, err
	}

	r.tasks[id] = s

	return s, err
}

func (r *Runtime) Get(ctx context.Context, id string) (runtime.Task, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.tasks[id]
	if !ok {
		return nil, ErrTaskNotExists
	}
	return t, nil
}

func (r *Runtime) Tasks(ctx context.Context) ([]runtime.Task, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	o := make([]runtime.Task, 0, len(r.tasks))
	for _, t := range r.tasks {
		o = append(o, t)
	}
	return o, nil
}

func (r *Runtime) Delete(ctx context.Context, c runtime.Task) (*runtime.Exit, error) {
	return &runtime.Exit{
		Status:    0,
		Timestamp: time.Now(),
	}, nil
}

func (r *Runtime) newBundle(id string, spec []byte) (string, error) {
	path := filepath.Join(r.root, id)
	if err := os.Mkdir(path, 0700); err != nil {
		return "", err
	}
	return path, nil
}

func (r *Runtime) deleteBundle(id string) error {
	return os.RemoveAll(filepath.Join(r.root, id))
}
