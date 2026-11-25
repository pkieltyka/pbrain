//go:build linux

package main

import (
	"fmt"
	"os"
	"strconv"

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
	if err := unix.IoctlSetPointerInt(fd, unix.TIOCSPTLCK, 0); err != nil {
		return fail("unlockpt", err)
	}
	n, err := unix.IoctlGetInt(fd, unix.TIOCGPTN)
	if err != nil {
		return fail("ptsname", err)
	}
	slave, err = os.OpenFile("/dev/pts/"+strconv.Itoa(n), os.O_RDWR|unix.O_NOCTTY, 0)
	if err != nil {
		return fail("open the terminal side", err)
	}
	tio, err := unix.IoctlGetTermios(int(slave.Fd()), unix.TCGETS)
	if err == nil {
		tio.Oflag &^= unix.OPOST
		err = unix.IoctlSetTermios(int(slave.Fd()), unix.TCSETS, tio)
	}
	if err != nil {
		slave.Close()
		return fail("termios", err)
	}
	return master, slave, nil
}
