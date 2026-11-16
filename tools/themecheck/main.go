// Command themecheck checks themes for the theme repository and writes its
// index.
//
//	go run ./tools/themecheck themes/harbor.zip themes/night.yml
//	go run ./tools/themecheck -index themes/*.yml themes/*.zip > index.json
//
// Each file is read as the application of this source reads a theme, and is
// held against the rules of the repository; see docs/theme-repository.md.
// The exit status is 1 if a theme does not meet them and 2 if the tool could
// not do its work.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/soulteary/flare/internal/themerepo"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run checks the files it is given. The report goes to report, and with
// -index the index of the themes goes to out, if every theme meets the
// rules.
func run(args []string, out, report io.Writer) int {
	flags := flag.NewFlagSet("themecheck", flag.ContinueOnError)
	flags.SetOutput(report)
	index := flags.Bool("index", false, "write the index of the themes to standard output")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() == 0 {
		say(report, "usage: themecheck [-index] <name>.yml|<name>.zip ...")
		return 2
	}
	// The files are read before the working directory is left.
	type input struct {
		file string
		raw  []byte
	}
	inputs := make([]input, 0, flags.NArg())
	for _, path := range flags.Args() {
		raw, err := os.ReadFile(path)
		if err != nil {
			say(report, "themecheck: "+err.Error())
			return 2
		}
		inputs = append(inputs, input{file: filepath.Base(path), raw: raw})
	}
	// A theme is stored and read in a directory that holds nothing else.
	work, err := os.MkdirTemp("", "flare-themecheck-")
	if err != nil {
		say(report, "themecheck: "+err.Error())
		return 2
	}
	defer func() {
		if removeErr := os.RemoveAll(work); removeErr != nil {
			say(report, "themecheck: "+removeErr.Error())
		}
	}()
	if err = os.Chdir(work); err != nil {
		say(report, "themecheck: "+err.Error())
		return 2
	}

	status := 0
	entries := make([]themerepo.Entry, 0, len(inputs))
	for _, input := range inputs {
		entry, findings := themerepo.Check(input.file, input.raw)
		entries = append(entries, entry)
		if len(findings) == 0 {
			note := ""
			if len(entry.Templates) > 0 {
				note = fmt.Sprintf(" (brings templates, to be read by a person: %v)", entry.Templates)
			}
			say(report, "ok    "+input.file+note)
			continue
		}
		status = 1
		for _, finding := range findings {
			say(report, "FAIL  "+input.file+": "+finding.Rule+": "+finding.Detail)
		}
	}
	listed, err := themerepo.NewIndex(entries)
	if err != nil {
		say(report, "FAIL  "+err.Error())
		status = 1
	}
	if !*index || status != 0 {
		return status
	}
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	if err = encoder.Encode(listed); err != nil {
		say(report, "themecheck: "+err.Error())
		return 2
	}
	return 0
}

// say writes a line of the report. A report that cannot be written has
// nowhere to say so.
func say(report io.Writer, line string) {
	if _, err := fmt.Fprintln(report, line); err != nil {
		return
	}
}
