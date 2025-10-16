package main

import (
	"github.com/spagettikod/vsix/cmd"
)

var version = "SET VERSION IN MAKEFILE"
var buildDate = "SET BUILD DATE IN MAKEFILE"

func main() {
	cmd.Execute(version, buildDate)
}
