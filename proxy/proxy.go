package proxy

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
)

// Proxy provides a way to programatically respond to invocations of a compiled
// binary that is created
type Proxy struct {
	// Ch is the channel of calls
	Ch chan *Call

	// Path is the full path to the compiled binproxy file
	Path string

	// A count of how many calls have been made
	CallCount int64

	// A temporary directory created for the binary
	tempDir string
}

// New returns a new instance of a Proxy with a compiled binary and a started server
func Compile(path string) (*Proxy, error) {
	var tempDir string

	if !filepath.IsAbs(path) {
		var err error
		tempDir, err = ioutil.TempDir("", "binproxy")
		if err != nil {
			return nil, fmt.Errorf("Error creating temp dir: %v", err)
		}
		path = filepath.Join(tempDir, path)
	}

	if runtime.GOOS == "windows" && !strings.HasSuffix(path, ".exe") {
		path += ".exe"
	}

	server, err := startServer()
	if err != nil {
		return nil, err
	}

	p := &Proxy{
		Path:    path,
		Ch:      make(chan *Call),
		tempDir: tempDir,
	}

	id, err := server.registerProxy(p)
	if err != nil {
		return nil, err
	}

	return p, compileClient(path, []string{
		"main.server=" + server.URL,
		"main.id=" + id,
	})
}

func (p *Proxy) newCall(args []string, env []string, dir string) *Call {
	return &Call{
		ID:         atomic.AddInt64(&p.CallCount, 1),
		Args:       args,
		Env:        env,
		Dir:        dir,
		exitCodeCh: make(chan int),
		doneCh:     make(chan struct{}),
	}
}

// Close the proxy and remove the temp directory
func (p *Proxy) Close() (err error) {
	close(p.Ch)

	defer func() {
		if p.tempDir != "" {
			if removeErr := os.RemoveAll(p.tempDir); removeErr != nil {
				err = removeErr
			}
		}
	}()
	defer func() {
		serverLock.Lock()
		defer serverLock.Unlock()
		if deregisterErr := serverInstance.deregisterProxy(p); deregisterErr != nil {
			err = deregisterErr
		}
	}()
	return err
}

// Call is created for every call to the proxied binary
type Call struct {
	sync.Mutex

	ID   int64
	Args []string
	Env  []string
	Dir  string

	// Stdout is the output writer to send stdout to in the proxied binary
	Stdout io.WriteCloser `json:"-"`

	// Stderr is the output writer to send stdout to in the proxied binary
	Stderr io.WriteCloser `json:"-"`

	// Stdin is the input reader for stdin from the proxied binary
	Stdin io.ReadCloser `json:"-"`

	// proxy      *Proxy
	exitCodeCh chan int
	doneCh     chan struct{}
}

func (c *Call) GetEnv(key string) string {
	for _, e := range c.Env {
		pair := strings.Split(e, "=")
		if strings.ToUpper(key) == strings.ToUpper(pair[0]) {
			return pair[1]
		}
	}
	return ""
}

// Exit finishes the call and the proxied binary returns the exit code
func (c *Call) Exit(code int) {
	_ = c.Stderr.Close()
	_ = c.Stdout.Close()

	// send the exit code to the server
	c.exitCodeCh <- code

	// wait for the client to get it
	<-c.doneCh
}

// Passthrough invokes another local binary and returns the results
func (c *Call) Passthrough(path string) {
	debugf("[server] Passing call through to %s %v", path, c.Args)

	cmd := exec.Command(path, c.Args...)
	cmd.Env = c.Env
	cmd.Stdout = c.Stdout
	cmd.Stderr = c.Stderr
	cmd.Stdin = c.Stdin
	cmd.Dir = c.Dir

	var waitStatus syscall.WaitStatus
	if err := cmd.Run(); err != nil {
		debugf("[server] Invoked command exited with error: %v", err)
		if exitError, ok := err.(*exec.ExitError); ok {
			waitStatus = exitError.Sys().(syscall.WaitStatus)
			c.Exit(waitStatus.ExitStatus())
		} else {
			panic(err)
		}
	} else {
		debugf("[server] Invoked command exited with 0")
		c.Exit(0)
	}
}

var (
	Debug bool
)

func debugf(pattern string, args ...interface{}) {
	if Debug {
		log.Printf(pattern, args...)
	}
}
