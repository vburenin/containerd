package vicruntime

import (
	"context"
	"io"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"os"

	"github.com/containerd/console"
	"github.com/containerd/containerd/plapi/client"
	"github.com/containerd/containerd/plugin"
	"github.com/containerd/fifo"
	"github.com/containerd/go-runc"
	"github.com/pkg/errors"
)

type VicProc struct {
	sync.WaitGroup

	// mu is used to ensure that `Start()` and `Exited()` calls return in
	// the right order when invoked in separate go routines.
	// This is the case within the shim implementation as it makes use of
	// the reaper interface.
	mu sync.Mutex
	pl *client.PortLayer

	console console.Console
	io      runc.IO
	exited  time.Time
	closers []io.Closer
	stdin   io.Closer

	stdinPath  string
	stdoutPath string
	stderrPath string
	terminal   bool
}

func NewVicProc(context context.Context, pl *client.PortLayer,
	path string, opts plugin.CreateOpts) (*VicProc, error) {

	p := &VicProc{
		pl:         pl,
		stdinPath:  opts.IO.Stdin,
		stdoutPath: opts.IO.Stdout,
		stderrPath: opts.IO.Stderr,
		terminal:   opts.IO.Terminal,
	}

	var (
		err    error
		socket *runc.Socket
		//		io     runc.IO
	)
	if p.terminal {
		if socket, err = runc.NewConsoleSocket(filepath.Join(path, "pty.sock")); err != nil {
			return nil, errors.Wrap(err, "failed to create console socket")
		}
		defer os.Remove(socket.Path())
	} else {
		//// TODO: get uid/gid
		//if io, err = runc.NewPipeIO(0, 0); err != nil {
		//	return nil, errors.Wrap(err, "failed to create runc io pipes")
		//}
		//p.io = io
	}
	if opts.Checkpoint != "" {
		//if _, err := p.runc.Restore(context, r.ID, r.Bundle, opts); err != nil {
		//	return nil, p.runcError(err, "runc restore failed")
		//}failed
	} else {

		//if socket != nil {
		//	opts.ConsoleSocket = socket
		//}
		//if err := p.runc.Create(context, r.ID, r.Bundle, opts); err != nil {
		//	return nil, p.runcError(err, "runc create failed")
		//}
	}
	if p.stdinPath != "" {
		sc, err := fifo.OpenFifo(context, p.stdinPath, syscall.O_WRONLY|syscall.O_NONBLOCK, 0)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to open stdin fifo %s", p.stdinPath)
		}
		p.stdin = sc
		p.closers = append(p.closers, sc)
	}
	if true {
		return p, nil
	}
	var copyWaitGroup sync.WaitGroup
	if socket != nil {
		console, err := socket.ReceiveMaster()
		if err != nil {
			return nil, errors.Wrap(err, "failed to retrieve console master")
		}
		p.console = console
		if err := copyConsole(context, console,
			p.stdinPath, p.stdoutPath, p.stderrPath, &p.WaitGroup, &copyWaitGroup); err != nil {
			return nil, errors.Wrap(err, "failed to start console copy")
		}
	} else {
		if err := copyPipes(context, nil, p.stdinPath, p.stdoutPath, p.stderrPath,
			&p.WaitGroup, &copyWaitGroup); err != nil {
			return nil, errors.Wrap(err, "failed to start io pipe copy")
		}
	}

	copyWaitGroup.Wait()
	if err != nil {
		return nil, errors.Wrap(err, "failed to retrieve runc container pid")
	}
	return p, nil
}

func (p *VicProc) Pid() int {
	return 1
}
