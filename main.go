package main

import (
	"github.com/soulteary/flare/cmd"
	"github.com/soulteary/flare/internal/server"
)

func main() {
	flags := cmd.Parse()
	server.StartDaemon(&flags)
}
