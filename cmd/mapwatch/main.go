package main

import "github.com/teochenglim/mapwatch/cmd"

// version is set at build time via -ldflags "-X main.version=<tag>".
// It defaults to the first release version.
var version = "0.1.0"

func main() {
	cmd.SetVersion(version)
	cmd.Execute()
}
