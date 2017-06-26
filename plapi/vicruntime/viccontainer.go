package vicruntime

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/containerd/console"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/plapi/client"
	"github.com/containerd/containerd/plapi/client/containers"
	"github.com/containerd/containerd/plapi/client/interaction"
	"github.com/containerd/containerd/plapi/client/logging"
	"github.com/containerd/containerd/plapi/client/scopes"
	"github.com/containerd/containerd/plapi/client/tasks"
	"github.com/containerd/containerd/plapi/models"
	"github.com/containerd/containerd/plapi/mounts"
	"github.com/containerd/containerd/plugin"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
)

type State struct {
	pid    uint32
	status plugin.Status
}

func (s State) Pid() uint32 {
	return s.pid
}

func (s State) Status() plugin.Status {
	return s.status
}

type VicTask struct {
	client *client.PortLayer
	comm   *plugin.IO
	con    console.Console

	tty bool
	mu  sync.Mutex

	events chan *plugin.Event

	id          string
	portLayerId string
}

const (
	AnnotationStdErrKey     = "containerd.stderr"
	AnnotationStdOutKey     = "containerd.stdout"
	AnnotationStdInKey      = "containerd.stdin"
	AnnotationImageID       = "containerd.img_id"
	AnnotationStorageName   = "containerd.storage_name"
	AnnotationContainerdID  = "containerd.id"
	AnnotationContainerSpec = "containerd.spec"
)

func loadSpec(specBin []byte) (*specs.Spec, error) {
	var spec specs.Spec
	if err := json.Unmarshal(specBin, &spec); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal oci spec")
	}
	return &spec, nil
}

type VicTasker struct {
	mu     sync.Mutex
	main   *VicProc
	other  map[int32]*VicTask
	events chan *plugin.Event
	pl     *client.PortLayer
	spec   *specs.Spec

	id      string
	vid     string
	root    string
	storage string
	imageID string
}

func NewVicTasker(ctx context.Context, pl *client.PortLayer, events chan *plugin.Event,
	storage, root, id string) *VicTasker {
	return &VicTasker{
		main:    nil,
		other:   make(map[int32]*VicTask),
		events:  events,
		id:      id,
		storage: storage,
		root:    root,
		pl:      pl,
	}
}

func (vt *VicTasker) Create(ctx context.Context, opts plugin.CreateOpts) error {
	spec, err := loadSpec(opts.Spec)

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
			AnnotationContainerSpec: string(opts.Spec),
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

	r1, err := vt.pl.Logging.LoggingJoin(
		logging.NewLoggingJoinParamsWithContext(ctx).WithConfig(
			&models.LoggingJoinConfig{
				Handle: handle,
			}))

	if err != nil {
		return err
	}

	handle = r1.Payload.Handle.(string)

	r2, err := vt.pl.Interaction.InteractionJoin(
		interaction.NewInteractionJoinParamsWithContext(ctx).WithConfig(
			&models.InteractionJoinConfig{
				Handle: handle,
			}))

	if err != nil {
		return err
	}

	handle = r2.Payload.Handle.(string)

	CommitHandle(ctx, vt.pl, handle)

	vt.events <- &plugin.Event{
		ID:        vt.id,
		Runtime:   runtimeName,
		Type:      plugin.CreateEvent,
		Timestamp: time.Now(),
		Pid:       1,
	}

	vicProc, err := NewVicProc(ctx, vt.pl, vt.root, opts)
	if err != nil {
		return err
	}

	vt.main = vicProc

	return nil
}

func (vt *VicTasker) Restore(ctx context.Context, ci *models.ContainerInfo) error {
	cfg := ci.ContainerConfig
	vt.storage = cfg.Annotations[AnnotationStorageName]
	vt.imageID = cfg.Annotations[AnnotationImageID]

	vt.vid = cfg.ContainerID
	vt.id = cfg.Annotations[AnnotationContainerdID]

	c := &VicTask{
		comm: &plugin.IO{
			Stdout:   cfg.Annotations[AnnotationStdOutKey],
			Stdin:    cfg.Annotations[AnnotationStdOutKey],
			Stderr:   cfg.Annotations[AnnotationStdErrKey],
			Terminal: cfg.Tty != nil && *cfg.Tty,
		},
	}

	if c.id == "" {
		c.id = c.portLayerId
	}
	return nil
}

func (vt *VicTasker) runIOServers(ctx context.Context) error {
	//var socket *runc.Socket
	//var err error
	//
	//if p.tty {
	//	if socket, err = runc.NewConsoleSocket(filepath.Join(p.root, p.id, "pty.sock")); err != nil {
	//		return err
	//	}
	//	con, err := socket.ReceiveMaster()
	//	if err != nil {
	//		return err
	//	}
	//
	//	p.con = con
	//
	//	go func() {
	//		stdInParams := interaction.NewContainerSetStdinParamsWithContext(ctx).WithID("tty-in")
	//		stdInParams.WithRawStream(con)
	//
	//		_, e := p.client.Interaction.ContainerSetStdin(stdInParams)
	//		if e != nil {
	//			log.G(ctx).Errorf("Could not setup stdin channel: %s", err)
	//		}
	//	}()
	//
	//	go func() {
	//		stdOutParams := interaction.NewContainerGetStdoutParamsWithContext(ctx).WithID("tty-out")
	//		_, e := p.client.Interaction.ContainerGetStdout(stdOutParams, con)
	//		if e != nil {
	//			log.G(ctx).Errorf("Could not setup stdout channel: %s", err)
	//		}
	//	}()
	//}

	return nil
}

func (vt *VicTasker) Info() plugin.TaskInfo {
	return plugin.TaskInfo{
		ID:          vt.id,
		ContainerID: vt.id,
		Runtime:     runtimeName,
		Spec:        nil,
	}
}

func (vt *VicTasker) Start(ctx context.Context) error {
	vt.mu.Lock()
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

	vt.events <- &plugin.Event{
		Pid:       1,
		ID:        vt.id,
		Timestamp: time.Now(),
		Type:      plugin.StartEvent,
		Runtime:   runtimeName,
	}
	return nil
}

func (vt *VicTasker) State(ctx context.Context) (plugin.State, error) {
	l := log.G(ctx)
	l.Debugf("State requested for container: %s", vt.id)
	vt.mu.Lock()
	defer vt.mu.Unlock()

	h, err := GetHandle(ctx, vt.pl, vt.vid)
	if err != nil {
		return plugin.State{}, err
	}

	r, err := vt.pl.Containers.GetState(containers.NewGetStateParamsWithContext(ctx).WithHandle(h))
	if err != nil {
		return plugin.State{}, err
	}

	status := plugin.Status(0)
	switch r.Payload.State {
	case "RUNNING":
		status = plugin.RunningStatus
	case "STOPPED":
		status = plugin.StoppedStatus
	case "CREATED":
		status = plugin.CreatedStatus
	default:
		status = plugin.Status(0)
	}

	l.Debugf("Container %s state: %s", vt.id, r.Payload.State)

	return plugin.State{
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

func (vt *VicTasker) Kill(ctx context.Context, signal, pid uint32, b bool) error {
	log.G(ctx).Debugf("Sending signal %d to %d: %s", signal, pid, vt.id)
	return nil
}

func (vt *VicTasker) Exec(ctx context.Context, opts plugin.ExecOpts) (plugin.Process, error) {
	return nil, fmt.Errorf("Exec is not implemented")
}

func (vt *VicTasker) Processes(ctx context.Context) ([]uint32, error) {
	return []uint32{1}, nil
}

func (vt *VicTasker) Pty(ctx context.Context, pid uint32, size plugin.ConsoleSize) error {
	log.G(ctx).Debugf("PTY requested for %d: %s", pid, vt.id)
	return nil
}

func (vt *VicTasker) CloseStdin(ctx context.Context, pid uint32) error {
	log.G(ctx).Debugf("Closing STDIN for %d: %s", pid, vt.id)
	return nil
}

func (vt *VicTasker) Checkpoint(ctx context.Context, opts plugin.CheckpointOpts) error {
	return fmt.Errorf("Check points are not supported yet")
}

func (vt *VicTasker) DeleteProcess(ctx context.Context, pid uint32) (*plugin.Exit, error) {
	log.G(ctx).Debugf("Deliting process %d: %s", pid, vt.id)
	return &plugin.Exit{
		Status:    0,
		Timestamp: time.Now(),
	}, nil
}
