package main

import (
	FlareCMD "github.com/soulteary/flare/cmd"
	FlareServer "github.com/soulteary/flare/internal/server"
)

func main() {
	flags := FlareCMD.Parse()
	FlareServer.StartDaemon(&flags)
}
