package vicruntime

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	events "github.com/containerd/containerd/api/services/events/v1"
	ctdevents "github.com/containerd/containerd/events"

	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/plapi/client"
	"github.com/containerd/containerd/plapi/client/containers"
	plevents "github.com/containerd/containerd/plapi/client/events"
	vwevents "github.com/containerd/containerd/plapi/events"
	"github.com/containerd/containerd/plapi/vicconfig"
	"github.com/containerd/containerd/plugin"
	"github.com/containerd/containerd/runtime"
	"github.com/containerd/containerd/typeurl"
	"github.com/go-openapi/swag"
	"github.com/pkg/errors"
)

const (
	runtimeName = "vmware-linux"
	storeName   = "containerd-storage"
)

var (
	ErrTaskNotExists     = errors.New("task does not exist")
	ErrTaskAlreadyExists = errors.New("task already exists")
	pluginID             = fmt.Sprintf("%s.%s", plugin.RuntimePlugin, runtimeName)
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

	events    chan interface{}
	monEvents chan interface{}

	emitter ctdevents.Poster

	pl            *client.PortLayer
	eventsContext context.Context
	eventsCancel  func()
	tasks         map[string]runtime.Task
	refMap        map[string]string
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
		events:        make(chan interface{}, 4096),
		monEvents:     make(chan interface{}, 4096),
		emitter:       ctdevents.GetPoster(ic.Context),
		eventsContext: c,
		eventsCancel:  cancel,
		pl:            PortLayerClient(cfg.PortlayerAddress),
		monitor:       monitor.(runtime.TaskMonitor),
		tasks:         make(map[string]runtime.Task),
		refMap:        make(map[string]string),
	}

	if err := r.updateContainerList(ic.Context); err != nil {
		return nil, err
	}

	r.startEventProcessor(c)

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

	for _, c := range cl.Payload {
		tid := c.ContainerConfig.Annotations[AnnotationContainerdID]
		refId := c.ContainerConfig.ContainerID
		vt := NewVicTasker(ctx, r.pl, r.events, storeName, r.root, tid)
		if err := vt.Restore(ctx, c); err != nil {
			log.G(ctx).Errorf("Failed to restore container %s, due to: %s", tid, err)
			continue
		}
		r.tasks[tid] = vt
		r.refMap[refId] = tid
	}
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
	path, err := r.newBundle(id, opts.Spec.Value)
	if err != nil {
		return nil, err
	}

	s := NewVicTasker(ctx, r.pl,
		r.events, "containerd-storage", path, id)

	if err := s.Create(ctx, opts); err != nil {
		os.RemoveAll(path)
		return nil, err
	}

	if err = r.monitor.Monitor(s); err != nil {
		return nil, err
	}

	r.tasks[id] = s
	r.refMap[s.vid] = s.id

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

func (r *Runtime) emit(ctx context.Context, topic string, evt interface{}) error {
	emitterCtx := ctdevents.WithTopic(ctx, topic)
	if err := r.emitter.Post(emitterCtx, evt); err != nil {
		return err
	}
	return nil
}

func (r *Runtime) adoptEvent(ctx context.Context, be *BaseEvent) {
	if be.Type != "events.ContainerEvent" {
		return
	}
	id, ok := r.refMap[be.Ref]
	if !ok {
		log.G(ctx).Warningf("Unknown event reference: %s", be.Ref)
		return
	}
	task, ok := r.tasks[id]
	if !ok {
		log.G(ctx).Warningf("Unknown task id %s for reference: %s", id, be.Ref)
		return
	}

	log.G(ctx).Debugf("Received container event: %s for %s(%s)", be.Event, be.Ref, id)
	switch be.Event {
	case vwevents.ContainerPoweredOff:
		r.events <- &events.TaskExit{
			ContainerID: id,
			Pid:         1,
			ExitStatus:  0,
			ExitedAt:    be.CreatedTime,
		}
		task.CloseIO(ctx)
	default:
		log.G(ctx).Warningf("Unknown event received: %s", be.Event)
	}
}

func (r *Runtime) startEventProcessor(ctx context.Context) {
	eventer := &Eventer{
		eventWriter: func(d []byte) (int, error) {
			log.G(ctx).Debugf("Event: %s", string(d))
			plEvent := &BaseEvent{}
			err := json.Unmarshal(d, &plEvent)
			if err != nil {
				log.G(ctx).Errorf("Received event is not decoded: %s, error: %s", string(d), err)
			}
			r.adoptEvent(ctx, plEvent)
			return len(d), nil
		},
	}
	go r.pl.Events.GetEvents(plevents.NewGetEventsParamsWithContext(ctx), eventer)

	go func() {
		for e := range r.events {
			// r.monEvents <- e
			a, err := typeurl.MarshalAny(e)
			if err != nil {
				log.G(ctx).WithError(err).Error("marshal event")
				return
			}

			topic := "/task/" + getTopic(e)
			ctx = ctdevents.WithTopic(ctx, topic)
			err = r.emitter.Post(ctx, &events.Envelope{
				Timestamp: time.Now(),
				Topic:     topic,
				Event:     a,
			})

			if err != nil {
				log.G(ctx).WithError(err).Error("post event")
			}
		}
		log.G(ctx).Infof("Exited event processor")
	}()
}

func getTopic(e interface{}) string {
	switch e.(type) {
	case *events.TaskCreate:
		return "task-create"
	case *events.TaskStart:
		return "task-start"
	case *events.TaskOOM:
		return "task-oom"
	case *events.TaskExit:
		return "task-exit"
	case *events.TaskDelete:
		return "task-delete"
	case *events.TaskExecAdded:
		return "task-exec-added"
	case *events.TaskPaused:
		return "task-paused"
	case *events.TaskResumed:
		return "task-resumed"
	case *events.TaskCheckpointed:
		return "task-checkpointed"
	}
	return "?"
}
