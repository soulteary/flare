//go:build !linux && !darwin

package cmd

import (
	"errors"
	"os"
)

func readTerminalPassword(f *os.File) (string, error) {
	return "", errors.New("hidden terminal input unavailable; use --password-file")
}
