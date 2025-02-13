package vscode

import (
	"slices"
	"testing"
)

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

func TestKeepVersions(t *testing.T) {
	type testCase struct {
		extension            Extension
		expectedVersionCount int
	}
	tests := []testCase{
		{
			extension: Extension{
				Name: "universal",
				Versions: []Version{
					{
						Version: "2.0",
					},
					{
						Version: "1.0",
					},
				},
			},
			expectedVersionCount: 1,
		},
		{
			extension: Extension{
				Name: "multiplatform",
				Versions: []Version{
					{
						Version:           "1.0",
						RawTargetPlatform: "linux-x64",
					},
					{
						Version:           "1.0",
						RawTargetPlatform: "darwin-x64",
					},
					{
						Version:           "1.0",
						RawTargetPlatform: "darwin-arm64",
					},
					{
						Version:           "0.9",
						RawTargetPlatform: "darwin-arm64",
					},
				},
			},
			expectedVersionCount: 3,
		},
	}
	for _, test := range tests {
		// fmt.Println(test.extension.LatestVersion(false))
		// fmt.Println(len(test.extension.Versions))
		ext := test.extension.KeepVersions(test.extension.LatestVersion(false))
		// fmt.Println(ext.LatestVersion(false))
		// fmt.Println(len(ext.Versions))
		if len(ext.Versions) != test.expectedVersionCount {
			t.Errorf("expected %v versions but got %v", test.expectedVersionCount, len(ext.Versions))
		}
	}
}

func TestSortFuncByInstallCount(t *testing.T) {
	exts := []Extension{
		{Name: "four", Statistics: []Statistic{{Name: string(StatisticInstall), Value: 4}}},
		{Name: "three", Statistics: []Statistic{{Name: string(StatisticInstall), Value: 3}}},
		{Name: "five", Statistics: []Statistic{{Name: string(StatisticInstall), Value: 5}}},
		{Name: "two", Statistics: []Statistic{{Name: string(StatisticInstall), Value: 2}}},
		{Name: "one", Statistics: []Statistic{{Name: string(StatisticInstall), Value: 1}}},
	}
	expected := "fivefourthreetwoone"
	slices.SortFunc(exts, SortFuncExtensionByInstallCount)
	actual := ""
	for _, e := range exts {
		actual += e.Name
	}
	if actual != expected {
		t.Fatalf("expected %s got %s", expected, actual)
	}
}
