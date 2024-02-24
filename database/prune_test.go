package database

import (
	"io/fs"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
)

func CompactPaths(paths []string) []string {
	slices.SortFunc(paths, func(a, b string) int {
		return len(a) - len(b)
	})

	shortest := []string{}
	for _, p1 := range paths {
		if !slices.ContainsFunc(shortest, func(p2 string) bool {
			return strings.HasPrefix(p1, p2)
		}) {
			shortest = append(shortest, p1)
		}
	}
	return shortest
}

func TestShortestPath(t *testing.T) {
	tests := []string{
		"a/b/c/d",
		"a/b/c/d/e",
		"g/h",
		"a/b",
		"a/b/c/d/e/f",
		"a/h/j/k",
		"a/h/j/k/l/m/n",
	}
	expected := []string{
		"g/h",
		"a/b",
		"a/h/j/k",
	}

	actual := CompactPaths(tests)
	if len(actual) != len(expected) {
		t.Fatalf("expected %v shortened paths but got %v", len(expected), len(actual))
	}
	for _, exp := range expected {
		if !slices.Contains(actual, exp) {
			t.Errorf("expected to find %s but did not", exp)
		}
	}
}

func TestClean(t *testing.T) {
	expectedKept := fstest.MapFS{
		"testdata/esbenp":                 &fstest.MapFile{Mode: fs.ModeDir},
		"testdata/esbenp/prettier-vscode": &fstest.MapFile{Mode: fs.ModeDir},
		"testdata/esbenp/prettier-vscode/_vsix_db_extension_metadata.json":                                       &fstest.MapFile{},
		"testdata/esbenp/prettier-vscode/0.40.3":                                                                 &fstest.MapFile{Mode: fs.ModeDir},
		"testdata/esbenp/prettier-vscode/0.40.3/1705943245285":                                                   &fstest.MapFile{Mode: fs.ModeDir},
		"testdata/esbenp/prettier-vscode/0.40.3/1705943245285/_vsix_db_version_metadata.json":                    &fstest.MapFile{},
		"testdata/esbenp/prettier-vscode/0.40.3/1705943245285/Microsoft.VisualStudio.Code.Manifest":              &fstest.MapFile{},
		"testdata/esbenp/prettier-vscode/0.40.3/1705943245285/Microsoft.VisualStudio.Services.Content.Changelog": &fstest.MapFile{},
		"testdata/publisher":                                                 &fstest.MapFile{Mode: fs.ModeDir},
		"testdata/publisher/extension":                                       &fstest.MapFile{Mode: fs.ModeDir},
		"testdata/publisher/extension/version":                               &fstest.MapFile{Mode: fs.ModeDir},
		"testdata/publisher/extension/version/version-id-with-only-metadata": &fstest.MapFile{Mode: fs.ModeDir},
		"testdata/publisher/extension/version/version-id-with-only-metadata/_vsix_db_version_metadata.json": &fstest.MapFile{},
		"testdata/publisher/extension/_vsix_db_extension_metadata.json":                                     &fstest.MapFile{},
		"testdata/publisher/extension-with-empty-version/_vsix_db_extension_metadata.json":                  &fstest.MapFile{},
		"testdata/publisher/extension-with-empty-version-id/_vsix_db_extension_metadata.json":               &fstest.MapFile{},
	}
	expectedToRemove := fstest.MapFS{
		"testdata/empty-publisher":                                                    &fstest.MapFile{Mode: fs.ModeDir},
		"testdata/non-valid-file":                                                     &fstest.MapFile{},
		"testdata/publisher/extension-without-metadata-file":                          &fstest.MapFile{Mode: fs.ModeDir},
		"testdata/publisher/extension-with-empty-version/empty-version":               &fstest.MapFile{Mode: fs.ModeDir},
		"testdata/publisher/extension-with-empty-version-id/version":                  &fstest.MapFile{Mode: fs.ModeDir},
		"testdata/publisher/extension-with-empty-version-id/version/empty-version-id": &fstest.MapFile{Mode: fs.ModeDir},
	}
	expectedOptional := fstest.MapFS{
		"testdata/publisher/extension-with-empty-version":    &fstest.MapFile{Mode: fs.ModeDir},
		"testdata/publisher/extension-with-empty-version-id": &fstest.MapFile{Mode: fs.ModeDir},
	}

	// add files to file system
	testFS := fstest.MapFS{}
	for k, v := range expectedKept {
		testFS[k] = v
	}
	for k, v := range expectedToRemove {
		testFS[k] = v
	}
	for k, v := range expectedOptional {
		testFS[k] = v
	}

	actual, err := process(testFS, "testdata", rootProcessor)
	if err != nil {
		t.Fatal(err)
	}

	if len(expectedKept) != len(actual.Kept) {
		t.Errorf("number of files expected to be kept does not match, expectedKept contained %v but expected %v", len(actual.Kept), len(expectedKept))
	}
	if len(expectedToRemove) != len(actual.Removed) {
		t.Errorf("number of files expected to be deleted does not match, expectedToRemove list contained %v but expected %v", len(actual.Removed), len(expectedToRemove))
	}
	if len(expectedOptional) != len(actual.Optional) {
		t.Errorf("number of files expected to be optional does not match, expectedOptional contained %v but expected %v", len(actual.Optional), len(expectedOptional))
	}
	// check we didn't miss removing any files
	for filename := range expectedToRemove {
		if !slices.Contains(actual.Removed, filename) {
			t.Errorf("expected %s to be marked for removal but it was not", filename)
		}
	}

	// check we didn't miss optional files
	for filename := range expectedOptional {
		if !slices.Contains(actual.Optional, filename) {
			t.Errorf("expected %s to be marked optional but it was not", filename)
		}
	}

	// check we didn't miss optional files
	for filename := range expectedKept {
		if !slices.Contains(actual.Kept, filename) {
			t.Errorf("expected %s to be kept but it was not", filename)
		}
	}

}
