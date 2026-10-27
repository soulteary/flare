//go:build darwin || linux

package data

import (
	"os"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExistingNavigationRejectsCachedFIFO(t *testing.T) {
	visibleNavigationFixture(t)
	_, _, err := ReadExistingVisibleNavigationPair(true, true, true)
	require.NoError(t, err)
	require.NoError(t, os.Remove("apps.yml"))
	require.NoError(t, syscall.Mkfifo("apps.yml", 0600))
	// No FIFO writer is present: opening it for reading would hang this test.
	_, err = ReadExistingVisibleFavoriteBookmarks(true)
	require.Error(t, err)
}
