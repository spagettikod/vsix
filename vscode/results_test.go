package vscode

import "testing"

func Test_Deduplication(t *testing.T) {
	exts := []Extension{
		{
			ID: "1",
		},
		{
			ID: "2",
		},
		{
			ID: "1",
		},
	}

	r := NewResults(exts)
	r.Deduplicate()

	if len(r.Results[0].Extensions) != 2 {
		t.Errorf("expected to find 2 extensions after deduplication but found %v", len(r.Results[0].Extensions))
	}
}

func Test_AddExtensions(t *testing.T) {
	exts := []Extension{
		{
			ID: "1",
		},
	}
	newExts := []Extension{
		{
			ID: "2",
		},
		{
			ID: "3",
		},
	}
	expectedIDs := []string{"1", "2", "3"}

	r := NewResults(exts)
	if len(r.Results[0].Extensions) != 1 {
		t.Errorf("Expected 1 extension but found %v", len(r.Results[0].Extensions))
	}
	r.AddExtensions(newExts)
	if len(r.Results[0].Extensions) != 3 {
		t.Errorf("Expected 3 extension but found %v", len(r.Results[0].Extensions))
	}
	for i := 0; i < 3; i++ {
		if r.Results[0].Extensions[i].ID != expectedIDs[i] {
			t.Errorf("Expected to find ID %v but found %v", r.Results[i].Extensions[i], expectedIDs[i])
		}
	}
}
