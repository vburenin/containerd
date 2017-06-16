package vicruntime

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/containerd/console"
	"github.com/containerd/containerd/api/types/mount"
	"github.com/containerd/containerd/plapi/client"
	"github.com/containerd/containerd/plapi/client/containers"
	"github.com/containerd/containerd/plapi/client/interaction"
	"github.com/containerd/containerd/plapi/client/logging"
	"github.com/containerd/containerd/plapi/models"
	"github.com/containerd/containerd/plugin"
	"github.com/containerd/go-runc"
	"github.com/go-openapi/swag"
	"google.golang.org/grpc"
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
	client   *client.PortLayer
	bundle   string
	storage  string
	imageID  string
	root     string
	shimSock string

	comm *plugin.IO
	con  console.Console

	tty      bool
	statusMu sync.Mutex

	server *grpc.Server
	events chan *plugin.Event

	mounts      []*mount.Mount
	id          string
	portLayerId string
}

const (
	AnnotationStdErrKey    = "containerd.stderr"
	AnnotationStdOutKey    = "containerd.stdout"
	AnnotationStdInKey     = "containerd.stdin"
	AnnotationImageID      = "containerd.img_id"
	AnnotationStorageName  = "containerd.storage_name"
	AnnotationContainerdID = "containerd.id"
)

func NewVicTask(ctx context.Context, portLayer *client.PortLayer,
	root, id string, opts plugin.CreateOpts) (plugin.Task, error) {

	c := &VicTask{
		client: portLayer,
		comm:   &opts.IO,
		bundle: filepath.Join(root, id),
		id:     id,
	}

	for _, v := range c.mounts {
		if v.Target == "" || v.Target == "/" {
			imagePath := strings.SplitN(v.Source, "/", -1)
			if len(imagePath) < 2 {
				continue
			}
			c.storage = imagePath[len(imagePath)-2]
			c.imageID = imagePath[len(imagePath)-1]
			break
		}
	}

	if c.imageID == "" || c.storage == "" {
		return nil, fmt.Errorf("No VM image id has been found")
	}

	_ = `
Mar  9 2017 10:37:00.655Z INFO  Create params. Name: %!s(*string=<nil>): {
   "annotations": {
     "containerd.img_id": "c40e708042c6a113e5827705cf9ace0377fae60cf9ae7a80ea8502d6460d87c6",
     "containerd.stderr": "/tmp/ctr/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa-676402324/stderr",
     "containerd.stdin": "/tmp/ctr/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa-676402324/stdin",
     "containerd.stdout": "/tmp/ctr/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa-676402324/stdout",
     "containerd.storage_name": "564df9f8-953e-0c9d-96a4-036e824e03cb"
   },
   "args": null,
   "attach": true,
   "env": [
     "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
     "TERM=xterm"
   ],
   "image": "c40e708042c6a113e5827705cf9ace0377fae60cf9ae7a80ea8502d6460d87c6",
   "imageStore": {
     "name": "564df9f8-953e-0c9d-96a4-036e824e03cb"
   },
   "memoryMB": 2048,
   "name": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
   "numCPUs": 2,
   "openStdin": true,
   "path": "/bin/bash",
   "repoName": "ubuntu",
   "stopSignal": "TERM",
   "tty": true,
   "workingDir": "/"
 }
 }`

	ccc := &models.ContainerCreateConfig{
		NumCpus:  2,
		MemoryMB: 2048,

		Image:    c.imageID, // request
		RepoName: "ubuntu",

		Name:            id,
		ImageStore:      &models.ImageStore{Name: c.storage},
		NetworkDisabled: false,
		Annotations: map[string]string{
			AnnotationStdInKey:     c.comm.Stdin,
			AnnotationStdOutKey:    c.comm.Stdout,
			AnnotationStdErrKey:    c.comm.Stderr,
			AnnotationImageID:      c.imageID,
			AnnotationStorageName:  c.storage,
			AnnotationContainerdID: id,
		},

		// Layer

	}
	logrus.Debugf("Creating new container: %s", id)
	logrus.Debugf("%#v", ccc)
	createParams := containers.NewCreateParamsWithContext(ctx).WithCreateConfig(ccc)

	r, err := c.client.Containers.Create(createParams)
	if err != nil {
		logrus.Warningf("Could not create container: %s", err)
		return nil, err
	}

	handle := r.Payload.Handle

	r1, err := c.client.Logging.LoggingJoin(
		logging.NewLoggingJoinParamsWithContext(ctx).WithConfig(
			&models.LoggingJoinConfig{
				Handle: handle,
			}))
	if err != nil {
		return nil, err
	}

	handle = r1.Payload.Handle.(string)

	r2, err := c.client.Interaction.InteractionJoin(
		interaction.NewInteractionJoinParamsWithContext(ctx).WithConfig(
			&models.InteractionJoinConfig{
				Handle: handle,
			}))

	if err != nil {
		return nil, err
	}
	handle = r2.Payload.Handle.(string)

	c.commitHandle(ctx, handle)
	c.portLayerId = r.Payload.ID

	if err := c.runIOServers(ctx); err != nil {
		return nil, err
	}

	return c, nil
}

func RestoreVicContaner(ctx context.Context, portlayer *client.PortLayer,
	root string, ci *models.ContainerInfo) plugin.Task {

	cfg := ci.ContainerConfig
	c := &VicTask{
		root:   root,
		client: portlayer,
		comm: &plugin.IO{
			Stdout:   cfg.Annotations[AnnotationStdOutKey],
			Stdin:    cfg.Annotations[AnnotationStdOutKey],
			Stderr:   cfg.Annotations[AnnotationStdErrKey],
			Terminal: cfg.Tty != nil && *cfg.Tty,
		},
		portLayerId: cfg.ContainerID,
		bundle:      cfg.LayerID,
		imageID:     cfg.Annotations[AnnotationImageID],
		storage:     cfg.Annotations[AnnotationStorageName],
		id:          cfg.Annotations[AnnotationContainerdID],
	}

	if c.id == "" {
		c.id = c.portLayerId
	}

	return c
}

func (p *VicTask) getHandle(ctx context.Context) (string, error) {
	r, err := p.client.Containers.Get(containers.NewGetParamsWithContext(ctx).WithID(p.portLayerId))
	if err != nil {
		logrus.Warningf("Could not get handle for container: %s", err)
		return "", fmt.Errorf("Could not get container handle: %s", p.id)
	}

	return r.Payload, nil
}

func (p *VicTask) commitHandle(ctx context.Context, h string) error {
	logrus.Debugf("Committing handle: %s", h)

	commitParams := containers.NewCommitParamsWithContext(ctx).WithHandle(h).WithWaitTime(swag.Int32(5))
	_, err := p.client.Containers.Commit(commitParams)
	if err != nil {
		logrus.Warningf("Could not commit handle %s: %s", h, err)
	}
	return err
}

func (p *VicTask) runIOServers(ctx context.Context) error {
	var socket *runc.Socket
	var err error

	if p.tty {
		if socket, err = runc.NewConsoleSocket(filepath.Join(p.root, p.id, "pty.sock")); err != nil {
			return err
		}
		con, err := socket.ReceiveMaster()
		if err != nil {
			return err
		}

		p.con = con

		go func() {
			stdInParams := interaction.NewContainerSetStdinParamsWithContext(ctx).WithID("tty-in")
			stdInParams.WithRawStream(con)

			_, e := p.client.Interaction.ContainerSetStdin(stdInParams)
			if e != nil {
				logrus.Errorf("Could not setup stdin channel: %s", err)
			}
		}()

		go func() {
			stdOutParams := interaction.NewContainerGetStdoutParamsWithContext(ctx).WithID("tty-out")
			_, e := p.client.Interaction.ContainerGetStdout(stdOutParams, con)
			if e != nil {
				logrus.Errorf("Could not setup stdout channel: %s", err)
			}
		}()
	}

	return nil
}

func (p *VicTask) Info() plugin.TaskInfo {
	return plugin.TaskInfo{
		ID:          p.id,
		ContainerID: p.id,
		Runtime:     runtimeName,
		Spec:        nil,
	}
}

func (p *VicTask) Start(ctx context.Context) error {
	h, err := p.getHandle(ctx)
	if err != nil {
		return fmt.Errorf("Container not found: %s", p.id)
	}

	logrus.Debugf("Starting container %s with handle %s", p.id, h)
	params := containers.NewStateChangeParamsWithContext(ctx)
	params.WithState("RUNNING").WithHandle(h)
	_, err = plClient.Containers.StateChange(params)
	if err != nil {
		return err
	}

	return p.commitHandle(ctx, h)
}

func (p *VicTask) State(ctx context.Context) (plugin.State, error) {
	p.statusMu.Lock()
	defer p.statusMu.Unlock()

	h, err := p.getHandle(ctx)
	if err != nil {
		return plugin.State{}, err
	}

	r, err := p.client.Containers.GetState(containers.NewGetStateParamsWithContext(ctx).WithHandle(h))
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

	//type State struct {
	//	// Status is the current status of the container
	//	Status
	//	// Pid is the main process id for the container
	//	Pid      uint32
	//	Stdin    string
	//	Stdout   string
	//	Stderr   string
	//	Terminal bool
	//}

	return plugin.State{
		Status:   status,
		Pid:      1,
		Stdin:    p.comm.Stdin,
		Stdout:   p.comm.Stdout,
		Stderr:   p.comm.Stderr,
		Terminal: p.tty,
	}, nil
}

func (p *VicTask) Pause(ctx context.Context) error {
	return nil
}

func (p *VicTask) Resume(ctx context.Context) error {
	return nil
}

func (p *VicTask) Kill(ctx context.Context, signal, pid uint32, b bool) error {
	return nil
}

func (p *VicTask) Exec(ctx context.Context, opts plugin.ExecOpts) (plugin.Process, error) {
	return nil, fmt.Errorf("Exec is not implemented")
}

func (p *VicTask) Processes(ctx context.Context) ([]uint32, error) {
	return []uint32{1}, nil
}

func (p *VicTask) Pty(ctx context.Context, pid uint32, size plugin.ConsoleSize) error {
	return nil
}

func (p *VicTask) CloseStdin(ctx context.Context, pid uint32) error {
	return nil
}

func (p *VicTask) Checkpoint(context.Context, plugin.CheckpointOpts) error {
	return fmt.Errorf("Check points are not supported yet")
}

func (p *VicTask) DeleteProcess(context.Context, uint32) (*plugin.Exit, error) {
	return &plugin.Exit{
		Status:    0,
		Timestamp: time.Now(),
	}, nil
}
