package main

import (
	"github.com/soulteary/flare/cmd"
	FlareServer "github.com/soulteary/flare/internal/server"
)

func main() {
	flags := cmd.Parse()
	FlareServer.StartDaemon(&flags)
}
