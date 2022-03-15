package vscode

import "testing"

func Test_VersionID(t *testing.T) {
	expected := "1644541363277"
	v := Version{AssetURI: "https://ms-vscode.gallerycdn.vsassets.io/extensions/ms-vscode/cpptools/1.9.0/1644541363277"}

	if v.ID() != expected {
		t.Errorf("expected %v but got %v", expected, v.ID())
	}
}

func Test_VersionCopy(t *testing.T) {
	test := Version{
		Files:      []Asset{{Source: "a", Path: "/b/c"}},
		Properties: []Property{{Key: "a", Value: "b"}},
	}
	result := test.Copy()
	if len(result.Files) != len(test.Files) {
		t.Errorf("expected %v files got %v", len(test.Files), len(result.Files))
	}
	if len(result.Properties) != len(test.Properties) {
		t.Errorf("expected %v properties got %v", len(test.Properties), len(result.Properties))
	}
	result.Files[0].Source = "FAIL"
	result.Files[0].Path = "FAIL"
	result.Properties[0].Key = "FAIL"
	result.Properties[0].Value = "FAIL"
	if (test.Files[0].Source != "a" ||
		test.Files[0].Path != "/b/c" ||
		test.Properties[0].Key != "a" ||
		test.Properties[0].Value != "b") &&
		(result.Files[0].Source == "FAIL" &&
			result.Files[0].Path == "FAIL" &&
			result.Properties[0].Key == "FAIL" &&
			result.Properties[0].Value == "FAIL") {
		t.Errorf("expected %v got %v", test, result)
	}
}
