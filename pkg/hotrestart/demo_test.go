package hotrestart_test

import (
	"net"
	"os"
	"strconv"
	"syscall"

	"github.com/cloudwego/kitex/pkg/hotrestart"
)

// var mockNetwork, mockPrimaryAddr = "tcp", ":8009"
var mockNetwork, mockPrimaryAddr = "unix", "test.primary.sock"

var mockAdminAddr = &net.UnixAddr{
	Net:  "unix",
	Name: "test.admin.sock",
}

var launcher = hotrestart.NewLauncher()

const RESTART = "RESTART"

func main() {
	s := os.Getenv(RESTART)
	times, err := strconv.Atoi(s)
	if err != nil {
		panic(err)
	}
	// os.Setenv(RESTART, strconv.Itoa(times))
	if times%2 == 0 {
		// svr 1
		run1()
		return
	}
	// svr2
	run2(launcher)
}

var defsvr = newSvr()

// svr 1
func run1() {
	if mockNetwork == "unix" {
		syscall.Unlink(mockPrimaryAddr)
		os.Chmod(mockPrimaryAddr, os.ModePerm)
	}
	if mockAdminAddr.Network() == "unix" {
		syscall.Unlink(mockAdminAddr.String())
		os.Chmod(mockAdminAddr.String(), os.ModePerm)
	}

	ln, err := net.Listen(mockNetwork, mockPrimaryAddr)
	if err != nil {
		panic(err)
	}
	err = launcher.Start(mockAdminAddr, ln, defsvr.Run, defsvr.Stop)
	if err != nil {
		panic(err)
	}

	// svr 1 dial
	conn, err := net.Dial(mockNetwork, mockPrimaryAddr)
	if err != nil {
		panic(err)
	}
	conn.Close()
	<-defsvr.quit
}

// svr 2
func run2(launcher hotrestart.Launcher) {
	err := launcher.Restart(mockAdminAddr, defsvr.Run, defsvr.Stop)
	if err != nil {
		panic(err)
	}
	// svr 2 dial
	conn, err := net.Dial(mockNetwork, mockPrimaryAddr)
	if err != nil {
		panic(err)
	}
	conn.Close()
	<-defsvr.quit
}

func newSvr() *svr {
	return &svr{
		quit: make(chan error, 1),
	}
}

type svr struct {
	quit chan error
	ln   net.Listener
}

func (s *svr) ListenFD(ln net.Listener) (fd int) {
	var f *os.File
	switch l := ln.(type) {
	case *net.TCPListener:
		f, _ = l.File()
	case *net.UnixListener:
		f, _ = l.File()
	}
	return int(f.Fd())
}

func (s *svr) FDListener(fd int) (net.Listener, error) {
	return hotrestart.RebuildListener(fd)
}

func (s *svr) Run(ln net.Listener) (err error) {
	s.ln = ln
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				s.quit <- err
				return
			}
			conn.Close()
		}
	}()
	return nil
}

func (s *svr) Stop() error {
	return s.ln.Close()
}
