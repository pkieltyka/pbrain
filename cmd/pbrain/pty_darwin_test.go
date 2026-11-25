//go:build darwin

package main

import (
	"fmt"
	"os"
	"unsafe"

	"golang.org/x/sys/unix"
)

// openPTYFiles opens a pseudo-terminal with output translation off, so a line feed reaches the
// reader as written.
func openPTYFiles() (master, slave *os.File, err error) {
	fd, err := unix.Open("/dev/ptmx", unix.O_RDWR|unix.O_NOCTTY|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, nil, fmt.Errorf("no pseudo-terminal: %w", err)
	}
	master = os.NewFile(uintptr(fd), "/dev/ptmx")
	fail := func(what string, err error) (*os.File, *os.File, error) {
		master.Close()
		return nil, nil, fmt.Errorf("%s: %w", what, err)
	}
	if err := unix.IoctlSetInt(fd, unix.TIOCPTYGRANT, 0); err != nil {
		return fail("grantpt", err)
	}
	if err := unix.IoctlSetInt(fd, unix.TIOCPTYUNLK, 0); err != nil {
		return fail("unlockpt", err)
	}
	var name [128]byte // TIOCPTYGNAME returns a NUL-terminated path.
	if _, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), unix.TIOCPTYGNAME, uintptr(unsafe.Pointer(&name[0]))); errno != 0 {
		return fail("ptsname", errno)
	}
	slave, err = os.OpenFile(unix.ByteSliceToString(name[:]), os.O_RDWR|unix.O_NOCTTY, 0)
	if err != nil {
		return fail("open the terminal side", err)
	}
	tio, err := unix.IoctlGetTermios(int(slave.Fd()), unix.TIOCGETA)
	if err == nil {
		tio.Oflag &^= unix.OPOST
		err = unix.IoctlSetTermios(int(slave.Fd()), unix.TIOCSETA, tio)
	}
	if err != nil {
		slave.Close()
		return fail("termios", err)
	}
	return master, slave, nil
}
