package marketplace

import "testing"

func TestEquals(t *testing.T) {
	tests := []ExtensionRequest{
		{
			UniqueID: "abcd",
		},
		{
			UniqueID: "abcd",
			Version:  "1.2.3",
		},
		{
			UniqueID:   "abcd",
			Version:    "1.2.3",
			PreRelease: true,
		},
		{
			UniqueID:        "abcd",
			Version:         "1.2.3",
			PreRelease:      true,
			TargetPlatforms: []string{"efgh", "ijkl"},
		},
	}
	for j := range tests {
		for i := range tests {
			if j == i && !tests[i].Equals(tests[j]) {
				t.Errorf("item %v should equal %v", i, j)
			} else if j != i && tests[i].Equals(tests[j]) {
				t.Errorf("item %v should not equal %v", i, j)
			}
		}
	}
}

func TestDeduplicate(t *testing.T) {
	tests := []ExtensionRequest{
		{
			UniqueID: "golang.Go",
		},
		{
			UniqueID: "ms-azuretools.vscode-docker",
		},
		{
			UniqueID: "ms-vscode.cpptools",
		},
		{
			UniqueID: "ms-vscode.cpptools",
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
