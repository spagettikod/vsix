package marketplace

import (
	"testing"

	"github.com/spagettikod/vsix/vscode"
)

// FIXME still relevant after rewrite?
// func TestEquals(t *testing.T) {
// 	tests := []ExtensionRequest{
// 		{
// 			UniqueID: "abcd",
// 		},
// 		{
// 			UniqueID: "abcd",
// 			Version:  "1.2.3",
// 		},
// 		{
// 			UniqueID:   "abcd",
// 			Version:    "1.2.3",
// 			PreRelease: true,
// 		},
// 		{
// 			UniqueID:        "abcd",
// 			Version:         "1.2.3",
// 			PreRelease:      true,
// 			TargetPlatforms: []string{"efgh", "ijkl"},
// 		},
// 	}
// 	for j := range tests {
// 		for i := range tests {
// 			if j == i && !tests[i].Equals(tests[j]) {
// 				t.Errorf("item %v should equal %v", i, j)
// 			} else if j != i && tests[i].Equals(tests[j]) {
// 				t.Errorf("item %v should not equal %v", i, j)
// 			}
// 		}
// 	}
// }

func TestDeduplicate(t *testing.T) {
	tests := []ExtensionRequest{
		{
			UniqueID: vscode.UniqueID{Publisher: "golang", Name: "Go"},
		},
		{
			UniqueID: vscode.UniqueID{Publisher: "ms-azuretools", Name: "vscode-docker"},
		},
		{
			UniqueID: vscode.UniqueID{Publisher: "ms-vscode", Name: "cpptools"},
		},
		{
			UniqueID: vscode.UniqueID{Publisher: "ms-vscode", Name: "cpptools"},
		},
	}
	result := Deduplicate(tests)
	if len(result) != 3 {
		t.Fatalf("expected 3 items, found %v", len(result))
	}
	if result[0].UniqueID != tests[0].UniqueID ||
		result[1].UniqueID != tests[1].UniqueID ||
		result[2].UniqueID != tests[2].UniqueID {
		t.Errorf("result %v, doesn't match expected results", result)
	}
}

func TestMatches(t *testing.T) {
	type TestCase struct {
		Request  ExtensionRequest
		Tag      vscode.VersionTag
		Expected bool
	}
	tests := []TestCase{
		{
			// 0
			Request:  ExtensionRequest{TargetPlatforms: []string{"darwin"}, PreRelease: false},
			Tag:      vscode.VersionTag{TargetPlatform: "darwin", PreRelease: false},
			Expected: true,
		},
		{
			// 1 true since pre-release also matches actual release
			Request:  ExtensionRequest{TargetPlatforms: []string{"darwin"}, PreRelease: true},
			Tag:      vscode.VersionTag{TargetPlatform: "darwin", PreRelease: false},
			Expected: true,
		},
		{
			// 2
			Request:  ExtensionRequest{TargetPlatforms: []string{"darwin", "win"}, PreRelease: false},
			Tag:      vscode.VersionTag{TargetPlatform: "darwin", PreRelease: false},
			Expected: true,
		},
		{
			// 3
			Request:  ExtensionRequest{TargetPlatforms: []string{"linux", "win"}, PreRelease: false},
			Tag:      vscode.VersionTag{TargetPlatform: "darwin", PreRelease: false},
			Expected: false,
		},
		{
			// 4
			Request:  ExtensionRequest{TargetPlatforms: []string{"linux", "win"}, PreRelease: true},
			Tag:      vscode.VersionTag{TargetPlatform: "darwin", PreRelease: false},
			Expected: false,
		},
		{
			// 5
			Request:  ExtensionRequest{TargetPlatforms: []string{"linux", "win"}, PreRelease: true},
			Tag:      vscode.VersionTag{TargetPlatform: "linux", PreRelease: true},
			Expected: true,
		},
	}
	for i, test := range tests {
		if test.Request.Matches(test.Tag) != test.Expected {
			t.Fatalf("test case #%v failed", i)
		}
	}
}
