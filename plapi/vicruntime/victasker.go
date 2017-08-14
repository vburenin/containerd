package vicruntime

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	eventsapi "github.com/containerd/containerd/api/services/events/v1"
	containerd_types "github.com/containerd/containerd/api/types"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/events"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/plapi/client"
	"github.com/containerd/containerd/plapi/client/containers"
	"github.com/containerd/containerd/plapi/client/interaction"
	"github.com/containerd/containerd/plapi/client/logging"
	"github.com/containerd/containerd/plapi/client/scopes"
	"github.com/containerd/containerd/plapi/client/tasks"
	"github.com/containerd/containerd/plapi/models"
	"github.com/containerd/containerd/plapi/mounts"
	"github.com/containerd/containerd/runtime"
	"github.com/gogo/protobuf/types"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
)

type State struct {
	pid    uint32
	status runtime.Status
}

func (s State) Pid() uint32 {
	return s.pid
}

func (s State) Status() runtime.Status {
	return s.status
}

const (
	AnnotationStdErrKey     = "containerd.stderr"
	AnnotationStdOutKey     = "containerd.stdout"
	AnnotationStdInKey      = "containerd.stdin"
	AnnotationImageID       = "containerd.img_id"
	AnnotationStorageName   = "containerd.storage_name"
	AnnotationContainerdID  = "containerd.id"
	AnnotationContainerSpec = "containerd.spec"
	AnnotationNamespace     = "containerd.namespace"
)

func loadSpec(specBin []byte) (*specs.Spec, error) {
	var spec specs.Spec
	if err := json.Unmarshal(specBin, &spec); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal oci spec")
	}
	return &spec, nil
}

type BaseEvent struct {
	Type        string
	Event       string
	ID          int
	Detail      string
	Ref         string
	CreatedTime time.Time
}

type Eventer struct {
	eventWriter func([]byte) (int, error)
}

func (e *Eventer) Write(d []byte) (int, error) {
	return e.eventWriter(d)
}

type VicTasker struct {
	mu      sync.Mutex
	main    *VicTask
	other   map[string]*VicTask
	events  events.Publisher
	pl      *client.PortLayer
	spec    *specs.Spec
	binSpec []byte

	id        string
	vid       string
	root      string
	storage   string
	imageID   string
	namespace string
}

func NewVicTasker(ctx context.Context, pl *client.PortLayer,
	events events.Publisher,
	namespace, storage, root, id string) *VicTasker {

	return &VicTasker{
		main:      nil,
		other:     make(map[string]*VicTask),
		events:    events,
		id:        id,
		storage:   storage,
		namespace: namespace,
		root:      root,
		pl:        pl,
	}
}

func (vt *VicTasker) ID() string {
	return vt.id
}

func (vt *VicTasker) Create(ctx context.Context, opts runtime.CreateOpts) error {
	vt.binSpec = opts.Spec.Value
	spec, err := loadSpec(vt.binSpec)

	if err != nil {
		return errors.Wrap(err, "Could not decode spec")
	}

	vt.spec = spec

	for _, v := range opts.Rootfs {
		log.G(ctx).Infof("Received mount: %#v", v)
		vmnt, err := mounts.ParseMountSource(v.Source)
		if err != nil {
			errors.Wrap(err, "Could not parse image mount")
		}
		vt.imageID = vmnt.Parent
		break
	}
	if vt.imageID == "" {
		return errors.New("No VM image id has been found")
	}

	ccc := &models.ContainerCreateConfig{
		Name: vt.id,

		NumCpus:  2,
		MemoryMB: 2048,

		Image:      vt.imageID, // request
		Layer:      vt.imageID,
		ImageStore: &models.ImageStore{Name: vt.storage},
		RepoName:   "ubuntu",

		NetworkDisabled: true,
		Annotations: map[string]string{
			AnnotationStdInKey:      opts.IO.Stdin,
			AnnotationStdOutKey:     opts.IO.Stdout,
			AnnotationStdErrKey:     opts.IO.Stderr,
			AnnotationImageID:       vt.imageID,
			AnnotationStorageName:   vt.storage,
			AnnotationContainerdID:  vt.id,
			AnnotationContainerSpec: string(opts.Spec.Value),
			AnnotationNamespace:     vt.namespace,
		},

		// Layer

	}
	log.G(ctx).Debugf("Creating new container: %s", vt.id)
	log.G(ctx).Debugf("%#v", ccc)
	createParams := containers.NewCreateParamsWithContext(ctx).WithCreateConfig(ccc)

	r, err := vt.pl.Containers.Create(createParams)
	if err != nil {
		log.G(ctx).Warningf("Could not create container: %s", err)
		return err
	}

	vt.vid = r.Payload.ID
	handle := r.Payload.Handle

	proc := spec.Process
	cfg := &models.TaskJoinConfig{
		ID:         vt.vid,
		Env:        proc.Env,
		WorkingDir: proc.Cwd,
		User:       proc.User.Username,
		Tty:        opts.IO.Terminal,
		OpenStdin:  opts.IO.Stdin != "",
		Attach:     true,
		Handle:     handle,
	}

	if len(proc.Args) > 0 {
		cfg.Path = proc.Args[0]
		cfg.Args = proc.Args[1:]
	}

	taskResp, err := vt.pl.Tasks.Join(tasks.NewJoinParamsWithContext(ctx).WithConfig(cfg))
	if err != nil {
		return err
	}

	handle = taskResp.Payload.Handle.(string)

	bindResp, err := vt.pl.Tasks.Bind(tasks.NewBindParamsWithContext(ctx).WithConfig(&models.TaskBindConfig{
		Handle: handle,
		ID:     vt.vid,
	}))
	if err != nil {
		return err
	}
	handle = bindResp.Payload.Handle.(string)

	r2, err := vt.pl.Interaction.InteractionJoin(
		interaction.NewInteractionJoinParamsWithContext(ctx).WithConfig(
			&models.InteractionJoinConfig{
				Handle: handle,
			}))

	if err != nil {
		return err
	}

	handle = r2.Payload.Handle.(string)

	r1, err := vt.pl.Logging.LoggingJoin(
		logging.NewLoggingJoinParamsWithContext(ctx).WithConfig(
			&models.LoggingJoinConfig{
				Handle: handle,
			}))

	if err != nil {
		return err
	}

	handle = r1.Payload.Handle.(string)

	CommitHandle(ctx, vt.pl, handle)

	var runtimeMounts []*containerd_types.Mount
	for _, m := range opts.Rootfs {
		runtimeMounts = append(runtimeMounts, &containerd_types.Mount{
			Type:    m.Type,
			Source:  m.Source,
			Options: m.Options,
			Target:  "/",
		})
	}

	e := &eventsapi.TaskCreate{
		ContainerID: vt.id,
		Bundle:      vt.root,
		Rootfs:      runtimeMounts,
		IO: &eventsapi.TaskIO{
			Stdin:    opts.IO.Stdin,
			Stdout:   opts.IO.Stdout,
			Stderr:   opts.IO.Stderr,
			Terminal: opts.IO.Terminal,
		},
		Checkpoint: opts.Checkpoint,
		Pid:        1,
	}

	vt.events.Publish(ctx, getTopic(e), e)

	vicProc := NewVicProc(vt.pl, vt.id, vt.vid, vt.root, opts)
	vt.main = vicProc

	return nil
}

func (vt *VicTasker) Restore(ctx context.Context, ci *models.ContainerInfo) error {
	cfg := ci.ContainerConfig
	vt.storage = cfg.Annotations[AnnotationStorageName]

	vt.imageID = cfg.Annotations[AnnotationImageID]
	vt.namespace = cfg.Annotations[AnnotationNamespace]
	vt.vid = cfg.ContainerID

	vt.main = &VicTask{
		terminal:   cfg.Tty != nil && *cfg.Tty,
		stdinPath:  cfg.Annotations[AnnotationStdInKey],
		stdoutPath: cfg.Annotations[AnnotationStdOutKey],
		stderrPath: cfg.Annotations[AnnotationStdErrKey],
		id:         cfg.Annotations[AnnotationContainerdID],
		vid:        cfg.ContainerID,
	}

	//vt.main.RunIO()

	return nil
}

func (vt *VicTasker) Info() runtime.TaskInfo {
	return runtime.TaskInfo{
		ID:        vt.id,
		Runtime:   pluginID,
		Spec:      vt.binSpec,
		Namespace: vt.namespace,
	}
}

func (vt *VicTasker) Start(ctx context.Context) error {
	vt.mu.Lock()
	defer vt.mu.Unlock()

	handle, err := GetHandle(ctx, vt.pl, vt.vid)
	if err != nil {
		return fmt.Errorf("Container not found: %s", vt.id)
	}

	bindResp, err := vt.pl.Scopes.BindContainer(scopes.NewBindContainerParamsWithContext(ctx).WithHandle(handle))
	if err != nil {
		return err
	}

	handle = bindResp.Payload.Handle

	log.G(ctx).Debugf("Starting container %s with handle %s", vt.id, handle)
	params := containers.NewStateChangeParamsWithContext(ctx).
		WithState("RUNNING").
		WithHandle(handle)

	stateResp, err := vt.pl.Containers.StateChange(params)
	if err != nil {
		return err
	}

	// resp.Payload is a returned new handle.
	if err = CommitHandle(ctx, vt.pl, stateResp.Payload); err != nil {
		return err
	}

	log.G(ctx).Infof("Container %s has started", vt.vid)

	err = vt.main.RunIO()
	if err != nil {
		log.G(ctx).Errorf("Failed to start container IO")
	}

	publishEvent(ctx, vt.events, &eventsapi.TaskStart{
		Pid:         1,
		ContainerID: vt.id,
	})
	return nil
}

func (vt *VicTasker) State(ctx context.Context) (runtime.State, error) {
	l := log.G(ctx)
	l.Debugf("State requested for container: %s, ref: %s", vt.id, vt.vid)

	vt.mu.Lock()
	defer vt.mu.Unlock()

	h, err := GetHandle(ctx, vt.pl, vt.vid)
	if err != nil {
		return runtime.State{}, err
	}

	r, err := vt.pl.Containers.GetState(containers.NewGetStateParamsWithContext(ctx).WithHandle(h))
	if err != nil {
		return runtime.State{}, err
	}

	status := runtime.Status(0)
	switch r.Payload.State {
	case "RUNNING":
		status = runtime.RunningStatus
	case "STOPPED":
		status = runtime.StoppedStatus
	case "CREATED":
		status = runtime.CreatedStatus
	default:
		status = runtime.Status(0)
	}

	l.Debugf("Container %s state: %s", vt.id, r.Payload.State)

	return runtime.State{
		Status:   status,
		Pid:      1,
		Stdin:    vt.main.stdinPath,
		Stdout:   vt.main.stdoutPath,
		Stderr:   vt.main.stderrPath,
		Terminal: vt.main.terminal,
	}, nil
}

func (vt *VicTasker) Pause(ctx context.Context) error {
	log.G(ctx).Debugf("Pausing container: %s", vt.id)
	return nil
}

func (vt *VicTasker) Resume(ctx context.Context) error {
	log.G(ctx).Debugf("Resuming container: %s", vt.id)
	return nil
}

func (vt *VicTasker) Kill(ctx context.Context, signal uint32, b bool) error {
	log.G(ctx).Debugf("Sending signal %d to %s", signal, vt.id)
	return nil
}

func (vt *VicTasker) Exec(ctx context.Context, s string, opts runtime.ExecOpts) (runtime.Process, error) {
	return nil, fmt.Errorf("Exec is not implemented")
}

func (vt *VicTasker) Pids(ctx context.Context) ([]uint32, error) {
	return []uint32{1}, nil
}

func (vt *VicTasker) ResizePty(ctx context.Context, size runtime.ConsoleSize) error {
	log.G(ctx).Debugf("PTY requested for %s, size %dx%d", vt.id, size.Width, size.Height)
	return vt.main.ResizePTY(ctx, size)
}

func (vt *VicTasker) CloseIO(ctx context.Context) error {
	log.G(ctx).Debugf("Closing IO for %s", vt.id)
	return vt.main.CloseIO(ctx)
}

func (vt *VicTasker) Checkpoint(ctx context.Context, cp string, any *types.Any) error {
	return fmt.Errorf("Check points are not supported yet")
}

func (vt *VicTasker) DeleteProcess(ctx context.Context, pid string) (*runtime.Exit, error) {
	log.G(ctx).Debugf("Deleting process %s: %s", pid, vt.id)
	return &runtime.Exit{
		Status:    0,
		Timestamp: time.Now(),
	}, nil
}

func (vt *VicTasker) Update(ctx context.Context, any *types.Any) error {
	return fmt.Errorf("Update is not supported yet")
}

func (vt *VicTasker) Process(ctx context.Context, pid string) (runtime.Process, error) {
	if pid == vt.id {
		return vt, nil
	}
	return nil, errors.Wrapf(errdefs.ErrNotFound, "Process not found: %s", pid)
}
