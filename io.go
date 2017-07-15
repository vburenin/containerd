package containerd

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
)

type IO struct {
	Terminal bool
	Stdin    string
	Stdout   string
	Stderr   string

	closer *wgCloser
}

func (i *IO) Cancel() {
	if i.closer == nil {
		return
	}
	i.closer.Cancel()
}

func (i *IO) Wait() {
	if i.closer == nil {
		return
	}
	i.closer.Wait()
}

func (i *IO) Close() error {
	if i.closer == nil {
		return nil
	}
	return i.closer.Close()
}

type IOCreation func() (*IO, error)

type IOAttach func(*FIFOSet) (*IO, error)

func NewIO(stdin io.Reader, stdout, stderr io.Writer) IOCreation {
	return NewIOWithTerminal(stdin, stdout, stderr, false)
}

func NewIOWithTerminal(stdin io.Reader, stdout, stderr io.Writer, terminal bool) IOCreation {
	return func() (*IO, error) {
		paths, err := NewFifos()
		if err != nil {
			return nil, err
		}
		i := &IO{
			Terminal: terminal,
			Stdout:   paths.Out,
			Stderr:   paths.Err,
			Stdin:    paths.In,
		}
		set := &ioSet{
			in:  stdin,
			out: stdout,
			err: stderr,
		}
		closer, err := copyIO(paths, set, i.Terminal)
		if err != nil {
			return nil, err
		}
		i.closer = closer
		return i, nil
	}

}

func WithAttach(stdin io.Reader, stdout, stderr io.Writer) IOAttach {
	return func(paths *FIFOSet) (*IO, error) {
		if paths == nil {
			return nil, fmt.Errorf("cannot attach to existing fifos")
		}
		i := &IO{
			Terminal: paths.Terminal,
			Stdout:   paths.Out,
			Stderr:   paths.Err,
			Stdin:    paths.In,
		}
		set := &ioSet{
			in:  stdin,
			out: stdout,
			err: stderr,
		}
		closer, err := copyIO(paths, set, i.Terminal)
		if err != nil {
			return nil, err
		}
		i.closer = closer
		return i, nil
	}
}

// Stdio returns an IO implementation to be used for a task
// that outputs the container's IO as the current processes Stdio
func Stdio() (*IO, error) {
	return NewIO(os.Stdin, os.Stdout, os.Stderr)()
}

// StdioTerminal will setup the IO for the task to use a terminal
func StdioTerminal() (*IO, error) {
	return NewIOWithTerminal(os.Stdin, os.Stdout, os.Stderr, true)()
}

// NewFifos returns a new set of fifos for the task
func NewFifos() (*FIFOSet, error) {
	root := filepath.Join(os.TempDir(), "containerd")
	if err := os.MkdirAll(root, 0700); err != nil {
		return nil, err
	}
	dir, err := ioutil.TempDir(root, "")
	if err != nil {
		return nil, err
	}
	return &FIFOSet{
		Dir: dir,
		In:  filepath.Join(dir, "stdin"),
		Out: filepath.Join(dir, "stdout"),
		Err: filepath.Join(dir, "stderr"),
	}, nil
}

type FIFOSet struct {
	// Dir is the directory holding the task fifos
	Dir          string
	In, Out, Err string
	Terminal     bool
}

type ioSet struct {
	in       io.Reader
	out, err io.Writer
}

type wgCloser struct {
	wg     *sync.WaitGroup
	dir    string
	set    []io.Closer
	cancel context.CancelFunc
}

func (g *wgCloser) Wait() {
	g.wg.Wait()
}

func (g *wgCloser) Close() error {
	for _, f := range g.set {
		f.Close()
	}
	if g.dir != "" {
		return os.RemoveAll(g.dir)
	}
	return nil
}

func (g *wgCloser) Cancel() {
	g.cancel()
}
