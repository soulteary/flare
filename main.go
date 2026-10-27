package main

import (
	"fmt"
	"github.com/soulteary/flare/cmd"
	"github.com/soulteary/flare/internal/server"
	"os"
)

func main() {
	if handled, code := cmd.ExecuteSecurityCommand(os.Args[1:], os.Stdin, os.Stdout, os.Stderr); handled {
		os.Exit(code)
	}
	flags := cmd.Parse()
	prepared := cmd.PrepareSecurity(&flags)
	if prepared.Store != nil {
		defer prepared.Store.Close()
	}
	if prepared.Status != "ready" {
		fmt.Fprintln(os.Stderr, "Flare security:", prepared.Status, prepared.Message)
	}
	server.StartDaemon(&flags, prepared.Store)
}
