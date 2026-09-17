//go:build darwin

package watch

import "golang.org/x/sys/unix"

// The ioctl request constants differ per OS (linux TCGETS/TCSETS, darwin
// TIOCGETA/TIOCSETA) — the release targets are linux and darwin only (D8).
func getTermios(fd int) (*unix.Termios, error) {
	return unix.IoctlGetTermios(fd, unix.TIOCGETA)
}

func setTermios(fd int, tio *unix.Termios) error {
	return unix.IoctlSetTermios(fd, unix.TIOCSETA, tio)
}
