package vicruntime

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	eventsapi "github.com/containerd/containerd/api/services/events/v1"
	ctdevents "github.com/containerd/containerd/events"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/plapi/client"
	"github.com/containerd/containerd/plapi/client/containers"
	plevents "github.com/containerd/containerd/plapi/client/events"
	vwevents "github.com/containerd/containerd/plapi/events"
	"github.com/containerd/containerd/plapi/vicconfig"
	"github.com/containerd/containerd/plugin"
	"github.com/containerd/containerd/runtime"
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

type contextEvent struct {
	ctx   context.Context
	event interface{}
}

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

	eventChan chan *contextEvent

	events *ctdevents.Exchange

	pl            *client.PortLayer
	eventsContext context.Context
	eventsCancel  func()
	tasks         *runtime.TaskList
	monitor       runtime.TaskMonitor

	// refMap is needed to keep pointers between PortLayer ID
	// and containerd ID to adopt messages from PortLayer which
	// container Portlayer ID.
	refMap map[string]VicRef
}

var _ runtime.Runtime = &Runtime{}

type VicRef struct {
	ID        string
	Namespace string
}

func New(ic *plugin.InitContext) (interface{}, error) {
	if err := os.MkdirAll(ic.Root, 0700); err != nil {
		return nil, err
	}

	monitor, err := ic.Get(plugin.TaskMonitorPlugin)
	if err != nil {
		return nil, err
	}

	cfg := ic.Config.(*vicconfig.Config)
	log.G(ic.Context).Infof("Vic runtime config: %q", cfg)

	c, cancel := context.WithCancel(ic.Context)
	r := &Runtime{
		root:          ic.Root,
		events:        ic.Events,
		eventsContext: c,
		eventsCancel:  cancel,
		pl:            PortLayerClient(cfg.PortlayerAddress),
		monitor:       monitor.(runtime.TaskMonitor),
		tasks:         runtime.NewTaskList(),
		refMap:        make(map[string]VicRef),
	}

	if err := r.updateContainerList(ic.Context); err != nil {
		return nil, err
	}

	r.startEventProcessor(c)

	return r, nil
}

func (r *Runtime) updateContainerList(ctx context.Context) error {
	log.G(ctx).Debugf("Refreshing running tasks list")

	r.mu.Lock()
	defer r.mu.Unlock()

	params := containers.NewGetContainerListParamsWithContext(ctx).WithAll(swag.Bool(true))

	cl, err := r.pl.Containers.GetContainerList(params)
	if err != nil {
		return err
	}
	log.G(ctx).Debugf("Discovered %d tasks", len(cl.Payload))

	for _, c := range cl.Payload {
		tid := c.ContainerConfig.Annotations[AnnotationContainerdID]
		namespace := c.ContainerConfig.Annotations[AnnotationNamespace]
		if namespace == "" {
			namespace = "default"
		}
		vt := NewVicTasker(
			ctx, r.pl, r.events, namespace,
			storeName, r.root, tid)
		if err := vt.Restore(ctx, c); err != nil {
			log.G(ctx).Errorf("Failed to restore container %s, due to: %s", tid, err)
			continue
		}
		if err := r.tasks.AddWithNamespace(namespace, vt); err != nil {
			log.G(ctx).WithError(err).Errorf("Could not add task")
		}
		r.refMap[vt.vid] = VicRef{ID: vt.id, Namespace: namespace}
	}
	return nil
}

func (r *Runtime) ID() string {
	return pluginID
}

func (r *Runtime) Create(ctx context.Context, id string, opts runtime.CreateOpts) (runtime.Task, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	namespace, err := namespaces.NamespaceRequired(ctx)

	log.G(ctx).Debugf("Starting runtime for %s. Options: %q", id, opts)
	path, err := r.newBundle(id, opts.Spec.Value)
	if err != nil {
		return nil, err
	}

	if _, err := r.tasks.Get(ctx, id); err == nil {
		return nil, runtime.ErrTaskAlreadyExists
	} else if err != runtime.ErrTaskNotExists {
		return nil, err
	}

	s := NewVicTasker(ctx, r.pl,
		r.events, namespace, "containerd-storage", path, id)

	if err := s.Create(ctx, opts); err != nil {
		os.RemoveAll(path)
		return nil, err
	}

	if err := r.tasks.Add(ctx, s); err != nil {
		return nil, err
	}

	r.refMap[s.vid] = VicRef{ID: s.id, Namespace: namespace}

	if err = r.monitor.Monitor(s); err != nil {
		return nil, err
	}

	return s, err
}

func (r *Runtime) Get(ctx context.Context, id string) (runtime.Task, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.tasks.Get(ctx, id)
}

func (r *Runtime) Tasks(ctx context.Context) ([]runtime.Task, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.tasks.GetAll(ctx)
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

func (r *Runtime) adoptEvent(ctx context.Context, be *BaseEvent) {
	if be.Type != "events.ContainerEvent" {
		return
	}
	ref, ok := r.refMap[be.Ref]

	if !ok {
		log.G(ctx).Warningf("Unknown event reference: %s", be.Ref)
		return
	}
	ctx = namespaces.WithNamespace(ctx, ref.Namespace)

	task, err := r.tasks.Get(ctx, ref.ID)
	if err != nil {
		log.G(ctx).Warningf("Event for an unknown container id: %s", ref.ID)
		return
	}

	log.G(ctx).Debugf(
		"Received container event: %s for %s(%s)",
		be.Event, be.Ref, ref.ID)
	switch be.Event {
	case vwevents.ContainerPoweredOff:
		e := &eventsapi.TaskExit{
			ContainerID: ref.ID,
			Pid:         1,
			ExitStatus:  0,
			ExitedAt:    be.CreatedTime,
		}
		publishEvent(ctx, r.events, e)
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

}

func publishEvent(ctx context.Context,
	publisher ctdevents.Publisher,
	event ctdevents.Event) {
	topic := getTopic(event)
	publisher.Publish(ctx, topic, event)
}

func getTopic(e interface{}) string {
	switch e.(type) {
	case *eventsapi.TaskCreate:
		return runtime.TaskCreateEventTopic
	case *eventsapi.TaskStart:
		return runtime.TaskStartEventTopic
	case *eventsapi.TaskOOM:
		return runtime.TaskOOMEventTopic
	case *eventsapi.TaskExit:
		return runtime.TaskExitEventTopic
	case *eventsapi.TaskDelete:
		return runtime.TaskDeleteEventTopic
	case *eventsapi.TaskExecAdded:
		return runtime.TaskExecAddedEventTopic
	case *eventsapi.TaskPaused:
		return runtime.TaskPausedEventTopic
	case *eventsapi.TaskResumed:
		return runtime.TaskResumedEventTopic
	case *eventsapi.TaskCheckpointed:
		return runtime.TaskCheckpointedEventTopic
	}
	return "?"
}
