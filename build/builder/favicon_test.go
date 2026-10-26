package builder

import (
	"bytes"
	"encoding/binary"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/config/model"
	"github.com/stretchr/testify/require"
)

func TestBuildThemeFavicons(t *testing.T) {
	raw, err := os.ReadFile("../../embed/assets/favicon.ico")
	require.NoError(t, err)
	icon, err := colorFavicon(raw, "#123456")
	require.NoError(t, err)
	require.EqualValues(t, 3, binary.LittleEndian.Uint16(icon[4:6]))
	for i, size := range []int{16, 32, 48} {
		entry := icon[6+i*16 : 6+(i+1)*16]
		require.EqualValues(t, size, entry[0])
		start := binary.LittleEndian.Uint32(entry[12:16])
		length := binary.LittleEndian.Uint32(entry[8:12])
		img, err := png.Decode(bytes.NewReader(icon[start : start+length]))
		require.NoError(t, err)
		require.Equal(t, size, img.Bounds().Dx())
		_, _, _, alpha := img.At(0, 0).RGBA()
		require.Zero(t, alpha, "outer corners must stay transparent")
		sourceEntry := raw[6+i*16 : 6+(i+1)*16]
		sourceStart := binary.LittleEndian.Uint32(sourceEntry[12:16])
		sourceLength := binary.LittleEndian.Uint32(sourceEntry[8:12])
		source, err := png.Decode(bytes.NewReader(raw[sourceStart : sourceStart+sourceLength]))
		require.NoError(t, err)
		found := false
		for y := 0; y < size; y++ {
			for x := 0; x < size; x++ {
				_, _, _, originalAlpha := source.At(x, y).RGBA()
				_, _, _, actualAlpha := img.At(x, y).RGBA()
				require.Equal(t, originalAlpha, actualAlpha, "artwork transparency must be preserved")
				if color.NRGBAModel.Convert(img.At(x, y)) == (color.NRGBA{R: 0x12, G: 0x34, B: 0x56, A: 255}) {
					found = true
				}
			}
		}
		require.True(t, found, "foreground must retain the original artwork")
	}
	dir := t.TempDir()
	dest := filepath.Join(dir, "favicon.ico")
	require.NoError(t, buildFavicons("../../embed/assets/favicon.ico", dest))
	first, err := os.ReadFile(filepath.Join(dir, "favicon_generated.go"))
	require.NoError(t, err)
	require.NoError(t, buildFavicons("../../embed/assets/favicon.ico", dest))
	second, err := os.ReadFile(filepath.Join(dir, "favicon_generated.go"))
	require.NoError(t, err)
	require.Equal(t, first, second)
	files, err := filepath.Glob(filepath.Join(dir, "favicons", "*.ico"))
	require.NoError(t, err)
	require.Len(t, files, len(define.ThemePalettes))
}

func TestDarkestPaletteColor(t *testing.T) {
	for _, test := range []struct {
		palette model.Palette
		want    string
	}{
		{model.Palette{Background: "#ffffff", Primary: "#222222", Accent: "#dddddd"}, "#222222"},
		{model.Palette{Background: "#1a1a1a", Primary: "#FFFDEA", Accent: "#5c5c5c"}, "#1a1a1a"},
		{model.Palette{Background: "#eeeeee", Primary: "#ffffff", Accent: "#101010"}, "#101010"},
		{model.Palette{Background: "#ff0000", Primary: "#00ff00", Accent: "#0000ff"}, "#0000ff"},
	} {
		got, err := darkestPaletteColor(test.palette)
		require.NoError(t, err)
		require.Equal(t, test.want, got)
	}
}
