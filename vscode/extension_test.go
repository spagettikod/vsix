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
