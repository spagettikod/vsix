package main

import (
	"os"
	"path"
	"strings"
	"testing"
)

func TestTemp(t *testing.T) {
	f, err := os.CreateTemp("temp", "vm1_*.part")
	if err != nil {
		t.Fatal(err)
	}
	name := f.Name()
	_, err = f.WriteString("test")
	if err != nil {
		t.Fatal(err)
	}
	err = f.Close()
	if err != nil {
		t.Fatal(err)
	}
	os.Rename(name, path.Join("temp", path.Base(name)[:strings.LastIndex(path.Base(name), ".part")]+".log"))
}
