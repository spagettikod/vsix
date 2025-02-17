package cmd

import (
	"slices"
	"testing"

	"github.com/spagettikod/vsix/vscode"
)

func TestFilterExtensions(t *testing.T) {
	exts := []vscode.Extension{
		{
			Publisher: vscode.Publisher{Name: "foo"},
			Name:      "bar",
			Versions: []vscode.Version{
				{
					RawTargetPlatform: "",
				},
			},
		},
		{
			Publisher: vscode.Publisher{Name: "foo"},
			Name:      "bar2",
			Versions: []vscode.Version{
				{
					RawTargetPlatform: "",
				},
			},
		},
		{
			Publisher: vscode.Publisher{Name: "foo"},
			Name:      "bar3",
			Versions: []vscode.Version{
				{
					RawTargetPlatform: "",
					Properties:        []vscode.Property{{Key: "Microsoft.VisualStudio.Code.PreRelease", Value: "true"}},
				},
			},
		},
		{
			Publisher: vscode.Publisher{Name: "foo"},
			Name:      "bar-darwin",
			Versions: []vscode.Version{
				{
					RawTargetPlatform: "darwin-arm64",
				},
			},
		},
		{
			Publisher: vscode.Publisher{Name: "foo"},
			Name:      "bar-linux",
			Versions: []vscode.Version{
				{
					RawTargetPlatform: "linux-x64",
				},
			},
		},
	}

	type Case struct {
		TargetPlatforms []string
		PreRelease      bool
		Prefix          string
		Expected        []string
	}
	testCases := []Case{
		{TargetPlatforms: []string{}, PreRelease: false, Prefix: "", Expected: []string{"foo.bar", "foo.bar2", "foo.bar3", "foo.bar-linux", "foo.bar-darwin"}},
		{TargetPlatforms: []string{}, PreRelease: true, Prefix: "", Expected: []string{"foo.bar3"}},
		{TargetPlatforms: []string{"darwin-arm64"}, PreRelease: false, Prefix: "", Expected: []string{"foo.bar-darwin"}},
		{TargetPlatforms: []string{"darwin-arm64", "linux-x64"}, PreRelease: false, Prefix: "", Expected: []string{"foo.bar-darwin", "foo.bar-linux"}},
		{TargetPlatforms: []string{}, PreRelease: false, Prefix: "foo.bar-", Expected: []string{"foo.bar-darwin", "foo.bar-linux"}},
	}

	for i, test := range testCases {
		fexts := filterExtensions(exts, test.TargetPlatforms, test.PreRelease, test.Prefix)
		actualUIDs := []string{}
		for _, ext := range fexts {
			actualUIDs = append(actualUIDs, ext.UniqueID().String())
		}
		slices.Sort(actualUIDs)
		slices.Sort(test.Expected)
		if slices.Compare(test.Expected, actualUIDs) != 0 {
			t.Fatalf("test case #%v failed, expected: %s got: %s", i, test.Expected, actualUIDs)
		}
	}
}
