package main

import (
	"github.com/segmentio/chamber/cmd"
	_ "github.com/segmentio/events/ecslogs"
)

func main() {
	cmd.Execute()
}
