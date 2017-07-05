package vicruntime

import (
	"context"
	"io"
	"sync"
	"syscall"
	"time"

	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/plapi/client"
	"github.com/containerd/containerd/plapi/client/interaction"
	"github.com/containerd/containerd/plapi/models"
	"github.com/containerd/containerd/plugin"
	"github.com/containerd/fifo"
	"github.com/pkg/errors"
)

type VicTask struct {
	id  string
	vid string

	consGroup sync.WaitGroup

	// mu is used to ensure that `Start()` and `Exited()` calls return in
	// the right order when invoked in separate go routines.
	// This is the case within the shim implementation as it makes use of
	// the reaper interface.
	mu sync.Mutex
	pl *client.PortLayer

	exited  time.Time
	closers []io.Closer
	stdin   io.Closer

	stdinPath  string
	stdoutPath string
	stderrPath string
	path       string
	terminal   bool
}

func NewVicProc(pl *client.PortLayer,
	id, vid string,
	path string, opts plugin.CreateOpts) *VicTask {

	return &VicTask{
		pl:         pl,
		stdinPath:  opts.IO.Stdin,
		stdoutPath: opts.IO.Stdout,
		stderrPath: opts.IO.Stderr,
		terminal:   opts.IO.Terminal,
		id:         id,
		vid:        vid,
		path:       path,
	}
}

func (p *VicTask) RunIO() error {
	ctx := context.Background()
	handle, err := GetHandle(ctx, p.pl, p.vid)
	if err != nil {
		return errors.Wrap(err, "Could not get container handle")
	}

	bindResp, err := p.pl.Interaction.InteractionBind(
		interaction.NewInteractionBindParamsWithContext(ctx).
			WithConfig(&models.InteractionBindConfig{
				Handle: handle,
				ID:     p.vid,
			}))

	if err != nil {
		return errors.Wrap(err, "Could not bind interaction channel")
	}

	handle = bindResp.Payload.Handle.(string)

	err = CommitHandle(ctx, p.pl, handle)
	if err != nil {
		return errors.Wrap(err, "Could not commit bind interaction handle")
	}

	if p.stdinPath != "" {
		ps, err := fifo.OpenFifo(ctx, p.stdinPath, syscall.O_RDONLY, 0)

		if err != nil {
			return err
		}

		p.closers = append(p.closers, ps)
		go func() {
			log.G(ctx).Debugf("Establishing STDIN connection: %s", p.vid)
			params := interaction.NewContainerSetStdinParamsWithContext(ctx).
				WithID(p.vid).
				WithRawStream(ps)

			_, err := p.pl.Interaction.ContainerSetStdin(params)

			if err != nil {
				log.G(ctx).Errorf("STDIN error happened: %s", err)
			}
			log.G(ctx).Debugf("STDIN connection has been closed: %s", p.vid)
		}()
	}

	if p.stdoutPath != "" {
		ps, err := fifo.OpenFifo(ctx, p.stdoutPath, syscall.O_WRONLY, 0)
		if err != nil {
			return err
		}
		p.consGroup.Add(1)
		go func() {
			defer ps.Close()
			defer p.consGroup.Done()
			log.G(ctx).Debugf("Establishing STDOUT connection: %s", p.vid)
			params := interaction.NewContainerGetStdoutParamsWithContext(ctx).WithID(p.vid)
			_, err := p.pl.Interaction.ContainerGetStdout(params, ps)
			if err != nil {
				log.G(ctx).Errorf("STDOUT error happened: %s", err)
			}
			log.G(ctx).Debugf("STDOUT connection has been closed: %s", p.vid)

		}()
	}

	if p.stderrPath != "" {
		ps, err := fifo.OpenFifo(ctx, p.stderrPath, syscall.O_WRONLY, 0)
		if err != nil {
			return err
		}
		p.consGroup.Add(1)
		go func() {
			defer ps.Close()
			defer p.consGroup.Done()
			params := interaction.NewContainerGetStderrParamsWithContext(ctx).WithID(p.vid)
			log.G(ctx).Debugf("Establishing STDERR connection: %s", p.id)
			_, err := p.pl.Interaction.ContainerGetStderr(params, ps)
			if err != nil {
				log.G(ctx).Errorf("STDERR error happened: %s", err)
			}
			log.G(ctx).Debugf("STDERR connection has been closed: %s", p.id)

		}()
	}

	log.G(ctx).Debugf("Container IO pipes are ready")

	return nil
}

func (p *VicTask) ResizePTY(ctx context.Context, size plugin.ConsoleSize) error {
	params := interaction.NewContainerResizeParamsWithContext(ctx).
		WithID(p.vid).
		WithWidth(int32(size.Width)).
		WithHeight(int32(size.Height))
	_, err := p.pl.Interaction.ContainerResize(params)
	if err != nil {
		err = errors.Wrap(err, "Could not resize PTY")
	}
	return err
}
func (p *VicTask) CloseSTDIN(ctx context.Context) error {
	for _, c := range p.closers {
		c.Close()
	}

	params := interaction.NewContainerCloseStdinParamsWithContext(ctx).WithID(p.vid)
	_, err := p.pl.Interaction.ContainerCloseStdin(params)
	if err != nil {
		err = errors.Wrap(err, "Could not close STDIN")
	}

	return err
}

func (p *VicTask) Pid() int {
	return 1
}
