package assets

import (
	"errors"
	"io"
	"io/fs"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
)

type failingStaticFS struct {
	file fs.File
	err  error
}

func (f failingStaticFS) Open(string) (fs.File, error) { return f.file, f.err }

type failingStaticFile struct {
	fs.File
	readErr, closeErr error
}

func (f failingStaticFile) Read([]byte) (int, error) { return 0, f.readErr }
func (f failingStaticFile) Close() error             { return f.closeErr }

type failingSeeker struct{ fs.File }

func (failingSeeker) Seek(int64, int) (int64, error) { return 0, fs.ErrInvalid }

func TestLazyStaticReaderPropagatesIOErrors(t *testing.T) {
	sentinel := errors.New("asset IO failure")
	for _, tc := range []struct {
		name  string
		files fs.FS
		want  error
	}{
		{"open", failingStaticFS{err: sentinel}, sentinel},
		{"read", failingStaticFS{file: failingStaticFile{readErr: sentinel}}, sentinel},
		{"close", failingStaticFS{file: failingStaticFile{readErr: io.EOF, closeErr: sentinel}}, sentinel},
		{"seek", failingStaticFS{file: failingSeeker{}}, fs.ErrInvalid},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := &lazyStaticReader{files: tc.files, name: "asset"}
			_, err := r.Read(make([]byte, 8))
			require.ErrorIs(t, err, tc.want)
		})
	}
	r := &lazyStaticReader{size: 6}
	_, err := r.Seek(-1, io.SeekStart)
	require.Error(t, err)
	_, err = r.Seek(0, 999)
	require.Error(t, err)
	offset, err := r.Seek(2, io.SeekCurrent)
	require.NoError(t, err)
	require.EqualValues(t, 2, offset)
	r.reader = failingSeeker{}
	_, err = r.Seek(0, io.SeekStart)
	require.ErrorIs(t, err, fs.ErrInvalid)
	require.EqualValues(t, 2, r.offset)
}

func TestStaticFSRejectsInvalidPathsAndDirectories(t *testing.T) {
	app := fiber.New()
	app.Use("/assets", StaticFS(fstest.MapFS{"dir": {Mode: fs.ModeDir}}, "/assets"))
	for _, tc := range []struct{ method, path string }{{"POST", "/assets/app.js"}, {"GET", "/assets/dir"}, {"GET", "/assets/a\\b"}} {
		r, err := app.Test(httptest.NewRequest(tc.method, tc.path, nil))
		require.NoError(t, err)
		require.Equal(t, 404, r.StatusCode)
		require.NoError(t, r.Body.Close())
	}
	broken := fiber.New()
	broken.Use(StaticFS(failingStaticFS{err: fs.ErrPermission}, ""))
	r, err := broken.Test(httptest.NewRequest("GET", "/asset", nil))
	require.NoError(t, err)
	defer r.Body.Close()
	require.Equal(t, 500, r.StatusCode)
}
