package FlareCMD_test

import (
	"flag"
	"os"
	"testing"

	FlareCMD "github.com/soulteary/flare/cmd"
	FlareDefine "github.com/soulteary/flare/config/define"
	"github.com/stretchr/testify/assert"
)

func TestGetCliFlags(t *testing.T) {
	originalArgs := os.Args
	defer func() { os.Args = originalArgs }()

	tests := []struct {
		name       string
		args       []string
		wantPort   int
		wantEnable bool
	}{
		{
			name:       "empty args",
			args:       []string{""},
			wantPort:   FlareDefine.DEFAULT_PORT,
			wantEnable: false,
		},
		{
			name:       "set port and enable guide",
			args:       []string{"app", "--port", "9090", "--enable-guide"},
			wantPort:   9090,
			wantEnable: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Args = append([]string{"app"}, tt.args...)
			flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
			gotFlags, _ := FlareCMD.GetCliFlags()
			assert.Equal(t, tt.wantPort, gotFlags.Port)
		})
	}
}

func TestGetFlagsMaps(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	tests := []struct {
		name string
		args []string
		want map[string]bool
	}{
		{
			name: "test single dash flags",
			args: []string{"cmd", "-foo", "-bar=value", "-baz"},
			want: map[string]bool{"foo": true, "bar": true, "baz": true},
		},
		{
			name: "test double dash flags",
			args: []string{"cmd", "--alpha", "--beta=ok", "--gamma"},
			want: map[string]bool{"alpha": true, "beta": true, "gamma": true},
		},
		{
			name: "test mixed dash flags",
			args: []string{"cmd", "--apple", "-banana=yellow", "--cherry", "-date"},
			want: map[string]bool{"apple": true, "banana": true, "cherry": true, "date": true},
		},
		{
			name: "test no flags",
			args: []string{"cmd"},
			want: map[string]bool{},
		},
		{
			name: "test empty args",
			args: []string{},
			want: map[string]bool{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Args = tt.args
			got := FlareCMD.GetFlagsMaps()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCheckFlagsExists(t *testing.T) {
	tests := []struct {
		name   string
		dict   map[string]bool
		keys   []string
		expect bool
	}{
		{
			name:   "all false",
			dict:   map[string]bool{"a": false, "b": false, "c": false},
			keys:   []string{"a", "b"},
			expect: false,
		},
		{
			name:   "one true",
			dict:   map[string]bool{"a": true, "b": false, "c": false},
			keys:   []string{"a", "b"},
			expect: true,
		},
		{
			name:   "none existent",
			dict:   map[string]bool{"a": true, "b": true},
			keys:   []string{"c", "d"},
			expect: false,
		},
		{
			name:   "empty keys",
			dict:   map[string]bool{"a": true, "b": true},
			keys:   []string{},
			expect: false,
		},
		{
			name:   "empty dict",
			dict:   map[string]bool{},
			keys:   []string{"a", "b"},
			expect: false,
		},
		{
			name:   "nil dict",
			dict:   nil,
			keys:   []string{"a", "b"},
			expect: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FlareCMD.CheckFlagsExists(tt.dict, tt.keys)
			assert.Equal(t, result, tt.expect)
		})
	}
}
