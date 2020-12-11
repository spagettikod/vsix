package main

import (
	"github.com/spagettikod/vsix/cmd"
)

var version = "SET VERSION IN MAKEFILE"

func main() {
	cmd.Execute(version)
}
