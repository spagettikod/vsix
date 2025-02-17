package vscode

import (
	"slices"
	"testing"
)

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

func TestMatchesTargetPlatform(t *testing.T) {
	exts := []Extension{
		{Versions: []Version{{RawTargetPlatform: ""}, {RawTargetPlatform: "web"}}},
	}
	type Case struct {
		TargetPlatforms []string
		Expected        bool
	}
	tests := []Case{
		{TargetPlatforms: []string{"darwin-arm64", "web"}, Expected: true},
		{TargetPlatforms: []string{"darwin-arm64"}, Expected: false},
		{TargetPlatforms: []string{"darwin-arm64", "linux-x64"}, Expected: false},
		{TargetPlatforms: []string{"universal"}, Expected: true},
	}

	for _, ext := range exts {
		for _, test := range tests {
			if ext.MatchesTargetPlatforms(test.TargetPlatforms...) != test.Expected {
				t.Fatalf("%s: failed", test.TargetPlatforms)
			}
		}
	}
}
