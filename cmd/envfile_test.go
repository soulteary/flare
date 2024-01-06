package FlareCMD_test

import (
	"os"
	"testing"

	FlareCMD "github.com/soulteary/flare/cmd"
	FlareDefine "github.com/soulteary/flare/config/define"
	FlareFn "github.com/soulteary/flare/internal/fn"
	"github.com/stretchr/testify/assert"
)

func TestCheckDotEnvFileExist(t *testing.T) {
	envPath := FlareFn.GetWorkDirFile(".env")

	// test .env not exist
	os.Remove(envPath)
	assert.Equal(t, FlareCMD.CheckDotEnvFileExist(envPath), false)

	// test .env exist
	f, _ := os.Create(envPath)
	filename := f.Name()
	defer os.Remove(filename)
	defer f.Close()
	assert.Equal(t, FlareCMD.CheckDotEnvFileExist(envPath), true)
}

func TestParseEnvFile_NotExist(t *testing.T) {
	os.Setenv("FLARE_DEBUG", "true")
	defer os.Unsetenv("FLARE_DEBUG")

	envParsed := FlareCMD.ParseEnvVars()
	envPath := FlareFn.GetWorkDirFile(".env")

	// test .env not exist
	os.Remove(envPath)
	flags := FlareCMD.ParseEnvFile(envParsed)
	assert.Equal(t, flags, envParsed)
}

func TestParseEnvFile_NotParsed(t *testing.T) {
	os.Setenv("FLARE_DEBUG", "true")
	defer os.Unsetenv("FLARE_DEBUG")

	envParsed := FlareCMD.ParseEnvVars()
	envPath := FlareFn.GetWorkDirFile(".env")

	// test .env not exist
	os.Mkdir(envPath, 0755)
	defer os.Remove(envPath)
	flags := FlareCMD.ParseEnvFile(envParsed)
	assert.Equal(t, flags, envParsed)
}

func TestParseEnvFile_ParseErr(t *testing.T) {
	os.Setenv("FLARE_DEBUG", "true")
	defer os.Unsetenv("FLARE_DEBUG")

	envParsed := FlareCMD.ParseEnvVars()
	envPath := FlareFn.GetWorkDirFile(".env")
	envParsed.Port = 1234
	envParsed.User = "123"
	envParsed.UserIsGenerated = true
	envParsed.Pass = "123"
	envParsed.PassIsGenerated = true

	// test .env auto correct
	f, _ := os.Create(envPath)
	defer os.Remove(envPath)
	defer f.Close()
	f.Write([]byte("FLARE_PORT=true\nFLARE_USER=\nFLARE_PASS=\nFLARE_GUIDE=1111"))
	flags := FlareCMD.ParseEnvFile(envParsed)
	assert.Equal(t, flags, envParsed)
}

func TestParseEnvFile_ParseOverwrite(t *testing.T) {
	os.Setenv("FLARE_DEBUG", "true")
	defer os.Unsetenv("FLARE_DEBUG")

	envParsed := FlareCMD.ParseEnvVars()
	envPath := FlareFn.GetWorkDirFile(".env")
	envParsed.Port = 1234
	envParsed.User = "123"
	envParsed.UserIsGenerated = true
	envParsed.Pass = "123"
	envParsed.PassIsGenerated = true

	// test .env auto correct
	f, _ := os.Create(envPath)
	defer os.Remove(envPath)
	defer f.Close()
	f.Write([]byte("FLARE_PORT=2345\nFLARE_USER=\nFLARE_PASS=\nFLARE_GUIDE=false"))
	flags := FlareCMD.ParseEnvFile(envParsed)

	envParsed.Port = 2345
	envParsed.EnableGuide = false

	assert.Equal(t, flags, envParsed)
}

func TestParseEnvFile_PortError(t *testing.T) {
	os.Setenv("FLARE_DEBUG", "true")
	defer os.Unsetenv("FLARE_DEBUG")

	envParsed := FlareCMD.ParseEnvVars()
	envPath := FlareFn.GetWorkDirFile(".env")
	envParsed.Port = 1234
	envParsed.User = "123"
	envParsed.UserIsGenerated = true
	envParsed.Pass = "123"
	envParsed.PassIsGenerated = true

	// test .env auto correct
	f, _ := os.Create(envPath)
	defer os.Remove(envPath)
	defer f.Close()
	f.Write([]byte("FLARE_PORT=9999999\nFLARE_USER=\nFLARE_PASS=\nFLARE_GUIDE=false"))
	flags := FlareCMD.ParseEnvFile(envParsed)

	envParsed.EnableGuide = false

	assert.Equal(t, flags, envParsed)
}

func TestParseEnvFile_User(t *testing.T) {
	os.Setenv("FLARE_DEBUG", "true")
	defer os.Unsetenv("FLARE_DEBUG")

	defaults := FlareDefine.DefaultEnvVars

	envParsed := FlareCMD.ParseEnvVars()
	envPath := FlareFn.GetWorkDirFile(".env")
	envParsed.User = defaults.User
	envParsed.Pass = defaults.Pass
	envParsed.UserIsGenerated = false
	envParsed.PassIsGenerated = false

	// test .env auto correct
	f, _ := os.Create(envPath)
	defer os.Remove(envPath)
	defer f.Close()
	f.Write([]byte("FLARE_PORT=5005\nFLARE_USER=flare\nFLARE_PASS=\n"))
	flags := FlareCMD.ParseEnvFile(envParsed)

	assert.Equal(t, flags, envParsed)
}
