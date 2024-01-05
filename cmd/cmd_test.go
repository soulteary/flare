package FlareCMD_test

import (
	"bytes"
	"io"
	"os"
	"testing"

	FlareCMD "github.com/soulteary/flare/cmd"
	FlareModel "github.com/soulteary/flare/config/model"
	"github.com/soulteary/flare/internal/version"
	flags "github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
)

func captureOutput(f func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	outC := make(chan string)
	go func() {
		var buf bytes.Buffer
		io.Copy(&buf, r)
		outC <- buf.String()
	}()
	f()
	w.Close()
	os.Stdout = old
	return <-outC
}

func TestExcuteCLI_ShowHelp(t *testing.T) {
	cliFlags := &FlareModel.Flags{ShowHelp: true}
	options := &flags.FlagSet{}

	output := captureOutput(func() {
		_ = FlareCMD.ExcuteCLI(cliFlags, options)
	})

	assert.Contains(t, output, "支持命令：", "应该打印出支持命令")
	assert.True(t, FlareCMD.ExcuteCLI(cliFlags, options), "在 ShowHelp 为 true 时，应该返回 true")
}

func TestExcuteCLI_ShowVersion(t *testing.T) {
	cliFlags := &FlareModel.Flags{ShowVersion: true}
	options := &flags.FlagSet{}

	output := captureOutput(func() {
		_ = FlareCMD.ExcuteCLI(cliFlags, options)
	})

	assert.Contains(t, output, version.Version, "应该打印出版本信息")
	assert.True(t, FlareCMD.ExcuteCLI(cliFlags, options), "在 ShowVersion 为 true 时，应该返回 true")
}

func TestExcuteCLI_NoFlags(t *testing.T) {
	cliFlags := &FlareModel.Flags{}
	options := &flags.FlagSet{}

	assert.False(t, FlareCMD.ExcuteCLI(cliFlags, options), "当没有任何标志被设置时，应该返回 false")
}

func TestGetVersionEcho(t *testing.T) {
	ver := ""
	// output := captureOutput(func() {
	ver = FlareCMD.GetVersion(true)
	// })
	assert.Contains(t, ver, version.Version, "应该打印出版本信息")
	// assert.Contains(t, output, "Challenge all bookmarking apps and websites directories, Aim to Be a best performance monster.", "应该打印详细信息")
}

func TestGetVersionMute(t *testing.T) {
	ver := ""
	output := captureOutput(func() {
		ver = FlareCMD.GetVersion(false)
	})
	assert.Contains(t, ver, version.Version, "应该打印出版本信息")
	assert.NotContains(t, output, "Challenge all bookmarking apps and websites directories, Aim to Be a best performance monster.", "不应该打印详细信息")
}
