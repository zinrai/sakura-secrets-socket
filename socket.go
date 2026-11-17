package main

import (
	"fmt"
	"log/slog"
	"net"
	"os"
	"syscall"
)

// ownerListener checks the peer uid, not only the socket mode: a mode
// loosened by hand would otherwise let every local user read every name.
type ownerListener struct {
	*net.UnixListener
	uid    uint32
	logger *slog.Logger
}

// listenOwner sets the umask around bind, not chmod after it: between bind
// and chmod the socket would be reachable with the default mode.
func listenOwner(path string, logger *slog.Logger) (*ownerListener, error) {
	// Checked here, not left to bind: bind reports only "invalid argument".
	if len(path) >= len(syscall.RawSockaddrUnix{}.Path) {
		return nil, fmt.Errorf("socket path %s is longer than %d bytes", path, len(syscall.RawSockaddrUnix{}.Path)-1)
	}
	old := syscall.Umask(0o177)
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	syscall.Umask(old)
	if err != nil {
		return nil, err
	}
	return &ownerListener{UnixListener: listener, uid: uint32(os.Getuid()), logger: logger}, nil
}

func (l *ownerListener) Accept() (net.Conn, error) {
	for {
		conn, err := l.AcceptUnix()
		if err != nil {
			return nil, err
		}
		cred, err := peerCred(conn)
		if err == nil && cred.Uid == l.uid {
			return conn, nil
		}
		l.logRefused(cred, err)
		conn.Close()
	}
}

// logRefused writes the error instead of the uid when the credentials could
// not be read, not a zero uid that would read as root.
func (l *ownerListener) logRefused(cred *syscall.Ucred, err error) {
	if err != nil {
		l.logger.Warn("refused", "error", err.Error())
		return
	}
	l.logger.Warn("refused", "uid", cred.Uid, "pid", cred.Pid)
}

func peerCred(conn *net.UnixConn) (*syscall.Ucred, error) {
	raw, err := conn.SyscallConn()
	if err != nil {
		return nil, err
	}
	var cred *syscall.Ucred
	var credErr error
	err = raw.Control(func(fd uintptr) {
		cred, credErr = syscall.GetsockoptUcred(int(fd), syscall.SOL_SOCKET, syscall.SO_PEERCRED)
	})
	if err != nil {
		return nil, err
	}
	return cred, credErr
}
