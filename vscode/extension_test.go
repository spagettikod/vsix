package vscode

import "testing"

func Test_ExtensionCopy(t *testing.T) {
	test := Extension{
		Versions: []Version{
			{
				Files:      []Asset{{Source: "a", Path: "/b/c"}},
				Properties: []Property{{Key: "a", Value: "b"}},
			}},
		Statistics: []Statistic{
			{Name: "stat", Value: 1},
		},
	}
	result := test.Copy()
	if len(result.Versions) != len(test.Versions) {
		t.Errorf("expected %v versions got %v", len(test.Versions), len(result.Versions))
	}
	if len(result.Statistics) != len(test.Statistics) {
		t.Errorf("expected %v statistics got %v", len(test.Statistics), len(result.Statistics))
	}
	result.Versions[0].Files[0].Source = "FAIL"
	result.Versions[0].Files[0].Path = "FAIL"
	result.Versions[0].Properties[0].Key = "FAIL"
	result.Versions[0].Properties[0].Value = "FAIL"
	if (test.Versions[0].Files[0].Source != "a" ||
		test.Versions[0].Files[0].Path != "/b/c" ||
		test.Versions[0].Properties[0].Key != "a" ||
		test.Versions[0].Properties[0].Value != "b") &&
		(result.Versions[0].Files[0].Source == "FAIL" &&
			result.Versions[0].Files[0].Path == "FAIL" &&
			result.Versions[0].Properties[0].Key == "FAIL" &&
			result.Versions[0].Properties[0].Value == "FAIL") {
		t.Errorf("expected %v got %v", test, result)
	}
}
