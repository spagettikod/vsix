package storage

import (
	"testing"

	"github.com/spagettikod/vsix/vscode"
)

func TestExtensionPath(t *testing.T) {
	expected := "redhat/java"
	id, ok := vscode.Parse("redhat.java")
	if !ok {
		t.Fatal("unexpected error parsing unique id")
	}
	tag := vscode.VersionTag{UniqueID: id}
	actual := ExtensionPath(tag.UniqueID)
	if actual != expected {
		t.Fatalf("expected %s but got %s", expected, actual)
	}
}

func TestAssetPath(t *testing.T) {
	expected := "redhat/java/1.2.3/darwin-arm64"
	id, ok := vscode.Parse("redhat.java")
	if !ok {
		t.Fatal("unexpected error parsing unique id")
	}
	tag := vscode.VersionTag{UniqueID: id, Version: "1.2.3", TargetPlatform: "darwin-arm64"}
	actual := AssetPath(tag)
	if actual != expected {
		t.Fatalf("expected %s but got %s", expected, actual)
	}
}
