package vscode

import "testing"

func Test_VersionID(t *testing.T) {
	expected := "1644541363277"
	v := Version{AssetURI: "https://ms-vscode.gallerycdn.vsassets.io/extensions/ms-vscode/cpptools/1.9.0/1644541363277"}

	if v.ID() != expected {
		t.Errorf("expected %v but got %v", expected, v.ID())
	}
}
