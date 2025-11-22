package vscode

import (
	"slices"
	"testing"
)

func TestSort(t *testing.T) {
	test := Versions{
		Version{Version: "2.0.0"},
		Version{Version: "1.0.0"},
		Version{Version: "3.0.0"},
		Version{Version: "0.2.0"},
	}
	expected := Versions{
		Version{Version: "3.0.0"},
		Version{Version: "2.0.0"},
		Version{Version: "1.0.0"},
		Version{Version: "0.2.0"},
	}
	test.Sort()
	equal := slices.CompareFunc(test, expected, func(v1, v2 Version) int {
		return v1.Compare(v2)
	}) == 0
	if !equal {
		t.Errorf("the two versions are not equal")
	}
}

func TestLatestVersion(t *testing.T) {
	test := Versions{
		Version{Version: "2.0.0"},
		Version{Version: "1.0.0"},
		Version{Version: "3.0.0"},
		Version{Version: "4.0.0", Properties: []Property{{Key: "Microsoft.VisualStudio.Code.PreRelease", Value: "true"}}},
		Version{Version: "0.2.0"},
	}
	expected := "3.0.0"
	expectedPreRelease := "4.0.0"
	actual := test.LatestVersion(false)
	if actual != expected {
		t.Errorf("expected version %s but got %s", expected, actual)
	}
	actual = test.LatestVersion(true)
	if actual != expectedPreRelease {
		t.Errorf("expected version %s but got %s", expectedPreRelease, actual)
	}
}

func TestLatest(t *testing.T) {
	test := Versions{
		Version{Version: "3.0.0"},
		Version{Version: "1.0.0"},
		Version{Version: "3.0.0"},
		Version{Version: "4.0.0", Properties: []Property{{Key: "Microsoft.VisualStudio.Code.PreRelease", Value: "true"}}},
		Version{Version: "0.2.0"},
	}
	expected := Versions{
		Version{Version: "3.0.0"},
		Version{Version: "3.0.0"},
	}
	expectedPreRelease := Versions{
		Version{Version: "4.0.0"},
		Version{Version: "3.0.0"},
		Version{Version: "3.0.0"},
	}
	equal := slices.CompareFunc(test.Latest(false), expected, func(v1, v2 Version) int {
		return v1.Compare(v2)
	}) == 0
	if !equal {
		t.Errorf("test failed")
	}
	equal = slices.CompareFunc(test.Latest(true), expectedPreRelease, func(v1, v2 Version) int {
		return v1.Compare(v2)
	}) == 0
	if !equal {
		t.Errorf("preRelease test failed")
	}
}
