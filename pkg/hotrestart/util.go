package hotrestart

import (
	"fmt"
	"net"
	"os"
)

func ListenFD(ln net.Listener) (fd int, f *os.File) {
	defer func() {
		if fd < 0 && f == nil {
			panic(fmt.Errorf("hot-restart can't parse listener, which type %T", ln))
		}
	}()
	if getter, ok := ln.(interface{ Fd() (fd int) }); ok {
		return getter.Fd(), nil
	}
	switch l := ln.(type) {
	case *net.TCPListener:
		f, _ = l.File()
	case *net.UnixListener:
		f, _ = l.File()
	}
	return int(f.Fd()), f
}

func RebuildListener(fd int) (net.Listener, error) {
	file := os.NewFile(uintptr(fd), "")
	if file == nil {
		return nil, fmt.Errorf("hot-restart failed to new file with fd %d", fd)
	}
	// can't close file here !
	ln, err := net.FileListener(file)
	if err != nil {
		return nil, err
	}
	return ln, nil
}
