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
	// new admin
	syscall.Unlink(admin.String())
	os.Chmod(admin.String(), os.ModePerm)
	l, err := net.Listen(admin.Network(), admin.String())
	if err != nil {
		return err
	}
	s.adminln, _ = l.(*net.UnixListener)

	// go run admin
	fds := []int{s.listenFD(ln), s.listenFD(s.adminln)}
	for i := range fds {
		syscall.SetNonblock(fds[i], true)
	}
	go s.listenAdmin(fds, ln, stop)

	// run svr
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
		return fmt.Errorf("hot restart rebuild fds[%d] != 2", len(fds))
	}

	// rebuild listener
	var ln net.Listener
	ln, err = RebuildListener(fds[0])
	if err != nil {
		return err
	}
	quit := make(chan error)
	go func() { quit <- run(ln) }()

	// rebuild admin
	var l net.Listener
	l, err = RebuildListener(fds[1])
	if err != nil {
		return err
	}
	s.adminln, _ = l.(*net.UnixListener)
	go s.listenAdmin(fds, l, stop)

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
		return nil, fmt.Errorf("hot-restart client write fd message: %w", err)
	}
	if readN != len(buf) {
		return nil, fmt.Errorf("hot-restart client write fd message: writeN mismatch")
	}
	rights := rightsBuf[:oobN]
	ctrlMsgs, err := syscall.ParseSocketControlMessage(rights)
	if err != nil {
		return nil, fmt.Errorf("hot-restart client parse socket control message: %w", err)
	}
	fds, err = syscall.ParseUnixRights(&ctrlMsgs[0])
	if err != nil {
		return nil, fmt.Errorf("hot-restart client parse unix rights: %w", err)
	}
	return fds, nil
}

func (s *launcher) rebuildAck(conn *net.UnixConn) (err error) {
	var buf [1]byte
	conn.SetDeadline(time.Now().Add(AdminTimeout))
	defer conn.Close()

	if _, err = conn.Write(buf[:]); err != nil {
		return fmt.Errorf("hot-restart client send termination: %w", err)
	}
	return nil
}

func (s *launcher) listenAdmin(fds []int, svrln net.Listener, stop Stop) (err error) {
	defer func() {
		if err != nil {
			panic(err)
		}
	}()
	for {
		var conn *net.UnixConn
		conn, err = s.adminln.AcceptUnix()
		if err != nil {
			return err
		}

		// Send FDs
		var buf [1]byte
		rights := syscall.UnixRights(fds...)
		conn.SetDeadline(time.Now().Add(AdminTimeout))
		var writeN, oobN int
		writeN, oobN, err = conn.WriteMsgUnix(buf[:], rights, nil)
		if err != nil {
			return fmt.Errorf("hot-restart server write fd message: %w", err)
		}
		if writeN != len(buf) {
			return fmt.Errorf("hot-restart server write fd message: writeN mismatch")
		}
		if oobN != len(rights) {
			return fmt.Errorf("hot-restart server write fd message: oobN mismatch")
		}

		// Wait for termination
		_ = conn.SetDeadline(time.Now().Add(AdminTimeout))
		_, err = conn.Read(buf[:])
		if err != nil {
			return fmt.Errorf("hot-restart server wait for termination: %w", err)
		}

		// quit
		err = s.linkQuit(svrln, stop)
		return err
	}
}

func (s *launcher) listenFD(ln net.Listener) (fd int) {
	var f *os.File
	fd, f = ListenFD(ln)
	s.files = append(s.files, f)
	return fd
}

func (s *launcher) linkQuit(svrln net.Listener, stop Stop) error {
	// if unix, not unlink
	switch l := svrln.(type) {
	case *net.UnixListener:
		l.SetUnlinkOnClose(false)
	}
	if s.adminln != nil {
		s.adminln.SetUnlinkOnClose(false)
	}
	stop()
	s.Shutdown()
	return nil
}
