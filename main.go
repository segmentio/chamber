package main

import (
	"time"

	"github.com/segmentio/chamber/cmd"
)

var (
	// This is updated by linker flags during build
	Version = "dev"
)

func main() {
	time.Sleep(5 * time.Second)
	cmd.Execute(Version)
}
