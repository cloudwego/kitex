package hotrestart

import (
	"fmt"
	"net"
	"os"
	"syscall"
	"time"
)

var AdminTimeout = time.Second

func NewLauncher() Launcher {
	restart := &launcher{}
	return restart
}

// Launcher will return run() finally, if success.
type Launcher interface {
	Start(admin *net.UnixAddr, ln net.Listener, run Run, stop Stop) error

	Restart(admin *net.UnixAddr, run Run, stop Stop) error

	Shutdown() error
}

type Run func(ln net.Listener) (err error)

type Stop func() error

var _ Launcher = &launcher{}

type launcher struct {
	adminln *net.UnixListener
	files   []*os.File
}

func (s *launcher) Shutdown() error {
	if s.adminln != nil {
		s.adminln.Close()
	}
	for i := range s.files {
		if s.files[i] != nil {
			s.files[i].Close()
		}
	}
	return nil
}

func (s *launcher) Start(admin *net.UnixAddr, ln net.Listener, run Run, stop Stop) error {
	fmt.Printf("DEBUG: run server addr=%s\n", ln.Addr().String())

	// new admin
	syscall.Unlink(admin.String())
	os.Chmod(admin.String(), os.ModePerm)
	adminln, err := net.Listen(admin.Network(), admin.String())
	if err != nil {
		return err
	}
	fmt.Printf("DEBUG: run admin addr=%s\n", adminln.Addr().String())
	go s.listenAdmin(ln, adminln, stop)

	// run svr
	switch svrln := ln.(type) {
	case *net.UnixListener:
		// if unix, not unlink
		svrln.SetUnlinkOnClose(false)
	}
	return run(ln)
}

func (s *launcher) Restart(admin *net.UnixAddr, run Run, stop Stop) error {
	conn, err := net.DialUnix(admin.Network(), nil, admin)
	if err != nil {
		return err
	}

	// get fds[ln.fd, admin.fd]
	var fds []int
	fds, err = s.rebuildFDs(conn)
	if err != nil {
		return err
	}
	if len(fds) != 2 {
		panic(fmt.Errorf("hot restart rebuild fds[%d] != 2", len(fds)))
	}
	if fds[0] < 0 || fds[1] < 0 {
		panic(fmt.Errorf("rebuild fds is invaild, fd[0]=%d, fd[1]=%d", fds[0], fds[1]))
	}

	// rebuild server listener
	var ln net.Listener
	ln, err = RebuildListener(fds[0])
	if err != nil {
		return err
	}
	fmt.Printf("DEBUG: rebuild server addr=%s\n", ln.Addr().String())
	quit := make(chan error)
	go func() { quit <- run(ln) }()

	// rebuild admin
	var adminln net.Listener
	adminln, err = RebuildListener(fds[1])
	if err != nil {
		panic(fmt.Errorf("rebuild admin listener failed: %s", err.Error()))
	}
	fmt.Printf("DEBUG: rebuild admin addr=%s\n", ln.Addr().String())

	go s.listenAdmin(ln, adminln, stop)

	// ack
	err = s.rebuildAck(conn)
	if err != nil {
		return err
	}
	return <-quit
}

func (s *launcher) rebuildFDs(conn *net.UnixConn) (fds []int, err error) {
	// Receive FDs
	var buf [1]byte
	var rightsBuf [1024]byte
	conn.SetDeadline(time.Now().Add(AdminTimeout))
	readN, oobN, _, _, err := conn.ReadMsgUnix(buf[:], rightsBuf[:])
	if err != nil {
		return nil, fmt.Errorf("hot-restart client write fd message: %s", err.Error())
	}
	if readN != len(buf) {
		return nil, fmt.Errorf("hot-restart client write fd message: writeN mismatch")
	}
	rights := rightsBuf[:oobN]
	ctrlMsgs, err := syscall.ParseSocketControlMessage(rights)
	if err != nil {
		return nil, fmt.Errorf("hot-restart client parse socket control message: %s", err.Error())
	}
	fds, err = syscall.ParseUnixRights(&ctrlMsgs[0])
	if err != nil {
		return nil, fmt.Errorf("hot-restart client parse unix rights: %s", err.Error())
	}
	return fds, nil
}

func (s *launcher) rebuildAck(conn *net.UnixConn) (err error) {
	var buf [1]byte
	conn.SetDeadline(time.Now().Add(AdminTimeout))
	defer conn.Close()

	if _, err = conn.Write(buf[:]); err != nil {
		return fmt.Errorf("hot-restart client send termination: %s", err.Error())
	}
	return nil
}

func (s *launcher) listenAdmin(svr, admin net.Listener, stop Stop) {
	// init
	adminln, ok := admin.(*net.UnixListener)
	if !ok {
		panic(fmt.Errorf("admin uds is not UnixListner, type=%T, l.Addr=%s", admin, admin.Addr()))
	}
	s.adminln = adminln
	s.adminln.SetUnlinkOnClose(false)

	fds := []int{s.listenFD(svr), s.listenFD(s.adminln)}
	for i := range fds {
		syscall.SetNonblock(fds[i], true)
	}

	// listen ...
	conn, err := s.adminln.AcceptUnix()
	if err != nil {
		panic(fmt.Errorf("hot-restart admin accept conn failed: %s\n", err.Error()))
	}

	// Send FDs
	var buf [1]byte
	rights := syscall.UnixRights(fds...)
	conn.SetDeadline(time.Now().Add(AdminTimeout))
	var writeN, oobN int
	writeN, oobN, err = conn.WriteMsgUnix(buf[:], rights, nil)
	if err != nil {
		panic(fmt.Errorf("hot-restart server write fd message: %s", err.Error()))
	}
	if writeN != len(buf) {
		panic(fmt.Errorf("hot-restart server write fd message: writeN mismatch"))
	}
	if oobN != len(rights) {
		panic(fmt.Errorf("hot-restart server write fd message: oobN mismatch"))
	}

	// Wait for termination
	_ = conn.SetDeadline(time.Now().Add(AdminTimeout))
	_, err = conn.Read(buf[:])
	if err != nil {
		panic(fmt.Errorf("hot-restart server wait for termination: %s", err.Error()))
	}

	// quit
	stop()
	s.Shutdown()
	return
}

func (s *launcher) listenFD(ln net.Listener) (fd int) {
	var f *os.File
	fd, f = ListenFD(ln)
	s.files = append(s.files, f)
	return fd
}
