package vicruntime

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/containerd/console"
	"github.com/containerd/containerd/api/types/container"
	"github.com/containerd/containerd/api/types/mount"
	"github.com/containerd/containerd/plapi/client"
	"github.com/containerd/containerd/plapi/client/containers"
	"github.com/containerd/containerd/plapi/client/interaction"
	"github.com/containerd/containerd/plapi/client/logging"
	"github.com/containerd/containerd/plapi/models"
	"github.com/containerd/containerd/plugin"
	"github.com/containerd/go-runc"
	"github.com/go-openapi/swag"
	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"
)

var protobufEmpty = &empty.Empty{}

const VicRuntimeName = "vmware"

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

type CommData struct {
	con console.Console

	stdin  string
	stdout string
	stderr string
	pid    int32
}

type VicContainer struct {
	client   *client.PortLayer
	bundle   string
	storage  string
	imageID  string
	root     string
	shimSock string

	initProc *CommData

	tty      bool
	statusMu sync.Mutex

	server *grpc.Server
	events chan *container.Event

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

func NewVicContainer(ctx context.Context, portLayer *client.PortLayer,
	root, id string, opts plugin.CreateOpts) (plugin.Container, error) {

	c := &VicContainer{}
	c.bundle = filepath.Join(root, id)
	c.id = id
	c.initProc.stdin = opts.IO.Stdin
	c.initProc.stdout = opts.IO.Stdout
	c.initProc.stderr = opts.IO.Stderr
	c.tty = opts.IO.Terminal
	c.client = portLayer

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
			AnnotationStdInKey:     c.initProc.stdin,
			AnnotationStdOutKey:    c.initProc.stdout,
			AnnotationStdErrKey:    c.initProc.stderr,
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

func RestoreVicContaner(ctx context.Context, portlayer *client.PortLayer, root string, ci *models.ContainerInfo) plugin.Container {
	c := &VicContainer{}
	c.client = portlayer
	c.initProc = &CommData{}

	cfg := ci.ContainerConfig
	c.portLayerId = cfg.ContainerID
	c.bundle = cfg.LayerID
	if cfg.Tty != nil {
		c.tty = *cfg.Tty
	}
	c.initProc.stdout = cfg.Annotations[AnnotationStdOutKey]
	c.initProc.stdin = cfg.Annotations[AnnotationStdOutKey]
	c.initProc.stderr = cfg.Annotations[AnnotationStdErrKey]

	c.imageID = cfg.Annotations[AnnotationImageID]
	c.storage = cfg.Annotations[AnnotationStorageName]

	c.id = cfg.Annotations[AnnotationContainerdID]
	if c.id == "" {
		c.id = c.portLayerId
	}

	return c
}

func (p *VicContainer) getHandle(ctx context.Context) (string, error) {
	r, err := p.client.Containers.Get(containers.NewGetParamsWithContext(ctx).WithID(p.portLayerId))
	if err != nil {
		logrus.Warningf("Could not get handle for container: %s", err)
		return "", fmt.Errorf("Could not get container handle: %s", p.id)
	}

	return r.Payload, nil
}

func (p *VicContainer) commitHandle(ctx context.Context, h string) error {
	logrus.Debugf("Commiting handle: %s", h)

	commitParams := containers.NewCommitParamsWithContext(ctx).WithHandle(h).WithWaitTime(swag.Int32(5))
	_, err := p.client.Containers.Commit(commitParams)
	if err != nil {
		logrus.Warningf("Could not commit handle %s: %s", h, err)
	}
	return err
}

func (p *VicContainer) runIOServers(ctx context.Context) error {
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
		p.initProc.con = con

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

func (p *VicContainer) Info() plugin.ContainerInfo {
	return plugin.ContainerInfo{
		ID:      p.id,
		Runtime: VicRuntimeName,
	}
}

func (p *VicContainer) Start(ctx context.Context) error {
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

func (p *VicContainer) State(ctx context.Context) (plugin.State, error) {
	p.statusMu.Lock()
	defer p.statusMu.Unlock()

	h, err := p.getHandle(ctx)
	if err != nil {
		return nil, err
	}

	r, err := p.client.Containers.GetState(containers.NewGetStateParamsWithContext(ctx).WithHandle(h))
	if err != nil {
		return nil, err
	}

	state := plugin.Status(0)
	switch r.Payload.State {
	case "RUNNING":
		state = plugin.RunningStatus
	case "STOPPED":
		state = plugin.StoppedStatus
	case "CREATED":
		state = plugin.CreatedStatus
	default:
		state = plugin.Status(0)
	}

	return &State{
		pid:    1,
		status: state,
	}, nil
}

func (p *VicContainer) Pause(ctx context.Context) error {
	return nil
}

func (p *VicContainer) Resume(ctx context.Context) error {
	return nil
}

func (p *VicContainer) Kill(ctx context.Context, signal, pid uint32, b bool) error {
	return nil
}

func (p *VicContainer) Exec(ctx context.Context, opts plugin.ExecOpts) (plugin.Process, error) {
	return nil, fmt.Errorf("Exec is not implemented")
}

func (p *VicContainer) Processes(ctx context.Context) ([]uint32, error) {
	return []uint32{1}, nil
}

func (p *VicContainer) Pty(ctx context.Context, pid uint32, size plugin.ConsoleSize) error {
	return nil
}

func (p *VicContainer) CloseStdin(ctx context.Context, pid uint32) error {
	return nil
}
