package vicruntime

import (
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/containerd/containerd/api/services/shim"
	"github.com/containerd/containerd/api/types/container"
	"github.com/containerd/containerd/api/types/mount"
	"github.com/containerd/containerd/plapi/client"
	"github.com/containerd/containerd/plapi/client/containers"
	"github.com/containerd/containerd/plapi/client/interaction"
	"github.com/containerd/containerd/plapi/client/logging"
	"github.com/containerd/containerd/plapi/models"
	"github.com/containerd/containerd/sys"
	"github.com/crosbymichael/console"
	"github.com/crosbymichael/go-runc"
	"github.com/go-openapi/swag"
	"github.com/golang/protobuf/ptypes/empty"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/grpclog"
)

var plonce sync.Once
var plClient *client.PortLayer

var protobufEmpty = &empty.Empty{}

func PortLayerClient() *client.PortLayer {
	plonce.Do(func() {
		cfg := client.DefaultTransportConfig().WithHost("172.16.165.130:2390")
		plClient = client.NewHTTPClientWithConfig(nil, cfg)
	})
	return plClient
}

type CommData struct {
	io      runc.IO
	console console.Console

	stdin  string
	stdout string
	stderr string
	pid    int32
}

type PortLayerShim struct {
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

func connectShim(socket string) (shim.ShimClient, error) {
	// reset the logger for grpc to log to dev/null so that it does not mess with our stdio
	grpclog.SetLogger(log.New(ioutil.Discard, "", log.LstdFlags))
	dialOpts := []grpc.DialOption{grpc.WithInsecure(), grpc.WithTimeout(100 * time.Second)}
	dialOpts = append(dialOpts,
		grpc.WithDialer(func(addr string, timeout time.Duration) (net.Conn, error) {
			return net.DialTimeout("unix", socket, timeout)
		}),
		grpc.WithBlock(),
		grpc.WithTimeout(2*time.Second),
	)
	conn, err := grpc.Dial(fmt.Sprintf("unix://%s", socket), dialOpts...)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to connect to shim via \"%s\"", fmt.Sprintf("unix://%s", socket))
	}
	return shim.NewShimClient(conn), nil
}

const (
	AnnotationStdErrKey    = "containerd.stderr"
	AnnotationStdOutKey    = "containerd.stdout"
	AnnotationStdInKey     = "containerd.stdin"
	AnnotationImageID      = "containerd.img_id"
	AnnotationStorageName  = "containerd.storage_name"
	AnnotationContainerdID = "containerd.id"
)

func basePortLayerShim(root, shimSock string) *PortLayerShim {
	return &PortLayerShim{
		root:     root,
		shimSock: shimSock,
		client:   PortLayerClient(),
		events:   make(chan *container.Event, 4096),
		initProc: &CommData{},
	}
}

func NewPortLayerShim(root string, shimSock string, containerInfo *models.ContainerInfo) (shim.ShimClient, error) {
	p := basePortLayerShim(root, shimSock)

	if err := p.restoreState(containerInfo); err != nil {
		return nil, err
	}

	if err := p.runShimServer(); err != nil {
		return nil, err
	}

	return connectShim(shimSock)
}

func (p *PortLayerShim) restoreState(ci *models.ContainerInfo) error {
	if ci != nil {
		cfg := ci.ContainerConfig
		p.portLayerId = cfg.ContainerID
		p.bundle = cfg.LayerID
		if cfg.Tty != nil {
			p.tty = *cfg.Tty
		}
		p.initProc.stdout = cfg.Annotations[AnnotationStdOutKey]
		p.initProc.stdin = cfg.Annotations[AnnotationStdOutKey]
		p.initProc.stderr = cfg.Annotations[AnnotationStdErrKey]

		p.imageID = cfg.Annotations[AnnotationImageID]
		p.storage = cfg.Annotations[AnnotationStorageName]

		p.id = cfg.Annotations[AnnotationContainerdID]
		if p.id == "" {
			p.id = p.portLayerId
		}
	}
	return nil
}

func (p *PortLayerShim) getHandle(ctx context.Context) (string, error) {
	r, err := p.client.Containers.Get(containers.NewGetParamsWithContext(ctx).WithID(p.portLayerId))
	if err != nil {
		logrus.Warningf("Could not get handle for container: %s", err)
		return "", fmt.Errorf("Could not get container handle: %s", p.id)
	}

	return r.Payload, nil
}

func (p *PortLayerShim) commitHandle(ctx context.Context, h string) error {
	logrus.Debugf("Commiting handle: %s", h)

	commitParams := containers.NewCommitParamsWithContext(ctx).WithHandle(h).WithWaitTime(swag.Int32(5))
	_, err := p.client.Containers.Commit(commitParams)
	if err != nil {
		logrus.Warningf("Could not commit handle %s: %s", h, err)
	}
	return err
}

func (p *PortLayerShim) runIOServers(ctx context.Context) error {
	var socket *runc.Socket
	var err error

	if p.tty {
		if socket, err = runc.NewConsoleSocket(filepath.Join(p.root, p.id, "pty.sock")); err != nil {
			return err
		}
		cons, err := socket.ReceiveMaster()
		if err != nil {
			return err
		}
		p.initProc.console = cons

		go func() {
			stdInParams := interaction.NewContainerSetStdinParamsWithContext(ctx).WithID("tty-in")
			stdInParams.WithRawStream(cons)

			_, e := p.client.Interaction.ContainerSetStdin(stdInParams)
			if e != nil {
				logrus.Errorf("Could not setup stdin channel: %s", err)
			}
		}()

		go func() {
			stdOutParams := interaction.NewContainerGetStdoutParamsWithContext(ctx).WithID("tty-out")
			_, e := p.client.Interaction.ContainerGetStdout(stdOutParams, cons)
			if e != nil {
				logrus.Errorf("Could not setup stdout channel: %s", err)
			}
		}()
	}

	return nil
}

func (p *PortLayerShim) runShimServer() error {
	// serve serves the grpc API over a unix socket at the provided path
	// this function does not block
	logrus.Debugf("Starting shim api at: %s", p.shimSock)
	l, err := sys.CreateUnixSocket(p.shimSock)
	if err != nil {
		return err
	}

	p.server = grpc.NewServer()
	shim.RegisterShimServer(p.server, p)

	go func() {
		defer l.Close()
		if err := p.server.Serve(l); err != nil {
			if !strings.Contains(err.Error(), "use of closed network") {
				logrus.WithError(err).Fatal("vic-shim: GRPC server failure")
			}
		}
	}()
	return nil
}

func (p *PortLayerShim) Create(ctx context.Context, in *shim.CreateRequest) (*shim.CreateResponse, error) {

	p.bundle = in.Bundle
	p.id = in.ID
	p.initProc.stdin = in.Stdin
	p.initProc.stdout = in.Stdout
	p.initProc.stderr = in.Stderr
	p.tty = in.Terminal
	p.mounts = in.Rootfs

	logrus.Println(in.String())

	fmt.Println()

	for _, v := range p.mounts {
		if v.Target == "" || v.Target == "/" {
			imagePath := strings.SplitN(v.Source, "/", -1)
			if len(imagePath) < 2 {
				continue
			}
			p.storage = imagePath[len(imagePath)-2]
			p.imageID = imagePath[len(imagePath)-1]
			break
		}
	}

	if p.imageID == "" || p.storage == "" {
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

		Image:    p.imageID, // request
		RepoName: "ubuntu",

		Name:            in.ID,
		ImageStore:      &models.ImageStore{Name: p.storage},
		NetworkDisabled: false,
		Annotations: map[string]string{
			AnnotationStdInKey:     p.initProc.stdin,
			AnnotationStdOutKey:    p.initProc.stdout,
			AnnotationStdErrKey:    p.initProc.stderr,
			AnnotationImageID:      p.imageID,
			AnnotationStorageName:  p.storage,
			AnnotationContainerdID: in.ID,
		},

		// Layer

	}
	logrus.Debugf("Creating new container: %s", in.ID)
	logrus.Debugf("%#v", ccc)
	createParams := containers.NewCreateParamsWithContext(ctx).WithCreateConfig(ccc)

	r, err := p.client.Containers.Create(createParams)
	if err != nil {
		logrus.Warningf("Could not create container: %s", err)
		return nil, err
	}

	handle := r.Payload.Handle

	r1, err := p.client.Logging.LoggingJoin(
		logging.NewLoggingJoinParamsWithContext(ctx).WithConfig(
			&models.LoggingJoinConfig{
				Handle: handle,
			}))
	if err != nil {
		return nil, err
	}

	handle = r1.Payload.Handle.(string)

	r2, err := p.client.Interaction.InteractionJoin(
		interaction.NewInteractionJoinParamsWithContext(ctx).WithConfig(
			&models.InteractionJoinConfig{
				Handle: handle,
			}))

	if err != nil {
		return nil, err
	}
	handle = r2.Payload.Handle.(string)

	p.commitHandle(ctx, handle)
	p.portLayerId = r.Payload.ID

	if err := p.runIOServers(ctx); err != nil {
		return nil, err
	}

	return &shim.CreateResponse{
		Pid: 1,
	}, nil
}

func (p *PortLayerShim) Start(ctx context.Context, in *shim.StartRequest) (*empty.Empty, error) {
	h, err := p.getHandle(ctx)
	if err != nil {
		return nil, fmt.Errorf("Container not found: %s", p.id)
	}

	logrus.Debugf("Starting container %s with handle %s", p.id, h)
	params := containers.NewStateChangeParamsWithContext(ctx)
	params.WithState("RUNNING").WithHandle(h)
	_, err = plClient.Containers.StateChange(params)
	if err != nil {
		return nil, err
	}
	p.commitHandle(ctx, h)
	return protobufEmpty, nil
}

func (p *PortLayerShim) Delete(ctx context.Context, in *shim.DeleteRequest) (*shim.DeleteResponse, error) {
	return &shim.DeleteResponse{
		ExitStatus: 0,
	}, nil
}

func (p *PortLayerShim) Exec(ctx context.Context, in *shim.ExecRequest) (*shim.ExecResponse, error) {
	panic("implement me")
}

func (p *PortLayerShim) Pty(ctx context.Context, in *shim.PtyRequest) (*empty.Empty, error) {
	return protobufEmpty, nil
}

func (p *PortLayerShim) Events(in *shim.EventsRequest, stream shim.Shim_EventsServer) error {
	for e := range p.events {
		if err := stream.Send(e); err != nil {
			return err
		}
	}
	return nil
}

func (p *PortLayerShim) Exit(ctx context.Context, in *shim.ExitRequest) (*empty.Empty, error) {
	return protobufEmpty, nil
}

func (p *PortLayerShim) Pause(ctx context.Context, in *shim.PauseRequest) (*empty.Empty, error) {
	return protobufEmpty, nil
}

func (p *PortLayerShim) Resume(ctx context.Context, in *shim.ResumeRequest) (*empty.Empty, error) {
	return protobufEmpty, nil
}

func (p *PortLayerShim) Kill(ctx context.Context, r *shim.KillRequest) (*empty.Empty, error) {
	//if r.Pid == 0 {
	//	if err := s.initProcess.Kill(ctx, r.Signal, r.All); err != nil {
	//		return nil, err
	//	}
	//	return empty, nil
	//}
	//proc, ok := s.processes[int(r.Pid)]
	//if !ok {
	//	return nil, fmt.Errorf("process does not exist %d", r.Pid)
	//}
	//if err := proc.Signal(int(r.Signal)); err != nil {
	//	return nil, err
	//}
	return protobufEmpty, nil
}

func (p *PortLayerShim) CloseStdin(ctx context.Context, r *shim.CloseStdinRequest) (*empty.Empty, error) {
	//p, ok := s.processes[int(r.Pid)]
	//if !ok {
	//	return nil, fmt.Errorf("process does not exist %d", r.Pid)
	//}
	//if err := p.Stdin().Close(); err != nil {
	//	return nil, err
	//}
	return protobufEmpty, nil
}

func (p *PortLayerShim) State(ctx context.Context, in *shim.StateRequest) (*shim.StateResponse, error) {
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

	state := container.Status_UNKNOWN

	switch r.Payload.State {
	case "RUNNING":
		state = container.Status_RUNNING
	case "STOPPED":
		state = container.Status_STOPPED
	case "CREATED":
		state = container.Status_CREATED
	default:
		state = container.Status_UNKNOWN
	}

	stateResp := &shim.StateResponse{
		ID:        p.id,
		Pid:       100,
		Bundle:    p.bundle,
		Processes: nil,
		Status:    state,
	}

	if state == container.Status_RUNNING {
		stateResp.Processes = append(stateResp.Processes, &container.Process{
			Pid:        100,
			Args:       nil,
			Cwd:        "/",
			Env:        nil,
			Terminal:   p.tty,
			ExitStatus: 0,
			Status:     container.Status_RUNNING,
		})
	}
	return stateResp, nil
}

//func (p *PortLayerShim) Start(ctx context.Context) error {
//
//}
//
//func (p *PortLayerShim) Info() containerd.ContainerInfo {
//	return containerd.ContainerInfo{
//		ID:      "id",
//		Runtime: "vic",
//	}
//}
//
//func (p *PortLayerShim) State(ctx context.Context) (containerd.State, error) {
//	return &VICContainerState{}, nil
//}
