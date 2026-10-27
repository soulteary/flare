//go:build darwin

package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"golang.org/x/sys/unix"
	"os"
	"strings"
)

func readTerminalPassword(f *os.File) (password string, err error) {
	fd := int(f.Fd())
	old, err := unix.IoctlGetTermios(fd, unix.TIOCGETA)
	if err != nil {
		return "", err
	}
	next := *old
	next.Lflag &^= unix.ECHO
	if err = unix.IoctlSetTermios(fd, unix.TIOCSETA, &next); err != nil {
		return "", err
	}
	defer func() {
		if restoreErr := unix.IoctlSetTermios(fd, unix.TIOCSETA, old); restoreErr != nil {
			password = ""
			err = errors.Join(err, fmt.Errorf("restore terminal settings: %w", restoreErr))
		}
	}()
	line, err := bufio.NewReader(f).ReadString('\n')
	return strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r"), err
}
