package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/midnattsol/docker-sweep/cmd"
	"github.com/midnattsol/docker-sweep/internal/docker"
)

var version = "dev"

func main() {
	// Docker CLI plugin metadata
	// When called as `docker sweep docker-cli-plugin-metadata`, return plugin info
	if len(os.Args) > 1 && os.Args[1] == "docker-cli-plugin-metadata" {
		meta := map[string]string{
			"SchemaVersion":    "0.1.0",
			"Vendor":           "midnattsol",
			"Version":          version,
			"ShortDescription": "Interactive Docker resource cleanup",
		}
		json.NewEncoder(os.Stdout).Encode(meta)
		return
	}

	// When called as Docker CLI plugin, Docker passes "sweep" as first arg
	// Remove it so cobra can parse the rest correctly
	if len(os.Args) > 1 && os.Args[1] == "sweep" {
		os.Args = append([]string{os.Args[0]}, os.Args[2:]...)
	}

	if err := docker.InitRuntime(os.Args[0]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	cmd.Execute(version)
}
