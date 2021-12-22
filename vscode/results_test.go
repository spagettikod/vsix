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
