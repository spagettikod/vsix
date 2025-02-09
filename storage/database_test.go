package storage

import (
	"testing"

	"github.com/spagettikod/vsix/vscode"
)

func TestExtensionPath(t *testing.T) {
	expected := "/data/redhat/java"
	db, err := OpenMem()
	if err != nil {
		t.Fatal("unexpected error opening database")
	}
	id, ok := vscode.Parse("redhat.java")
	if !ok {
		t.Fatal("unexpected error parsing unique id")
	}
	tag := vscode.ExtensionTag{UniqueID: id}
	actual := db.extensionPath(tag.UniqueID)
	if actual != expected {
		t.Fatalf("expected %s but got %s", expected, actual)
	}
}

func TestAssetPath(t *testing.T) {
	expected := "/data/redhat/java/1.2.3/darwin-arm64"
	db, err := OpenMem()
	if err != nil {
		t.Fatal("unexpected error opening database")
	}
	id, ok := vscode.Parse("redhat.java")
	if !ok {
		t.Fatal("unexpected error parsing unique id")
	}
	tag := vscode.ExtensionTag{UniqueID: id, Version: "1.2.3", TargetPlatform: "darwin-arm64"}
	actual := db.assetPath(tag)
	if actual != expected {
		t.Fatalf("expected %s but got %s", expected, actual)
	}
}
