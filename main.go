package main

import "github.com/segmentio/chamber/cmd"

var (
	// This is updated by linker flags during build
	Version           = "dev"
	AnalyticsWriteKey = ""
)

func main() {
	cmd.Execute(Version, AnalyticsWriteKey)
}
