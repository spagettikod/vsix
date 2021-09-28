package vscode

import (
	"encoding/json"
	"fmt"
	"mime"
	"net/url"
	"path"
	"testing"
)

func Test_t(t *testing.T) {
	u, _ := url.Parse("https://golang.gallerycdn.vsassets.io/extensions/golang/go/0.26.0/1623958451720/Microsoft.VisualStudio.Code.Manifest")

	fmt.Println(u.Host)
	fmt.Println(u.Path)
	fmt.Println(path.Split(u.Path))

	fmt.Println(mime.ExtensionsByType("application/json"))
}

func Test_a(t *testing.T) {
	exts, err := BuildDatabase("/home/roland/development/vsix/_data")
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.MarshalIndent(exts, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(string(b))
}
