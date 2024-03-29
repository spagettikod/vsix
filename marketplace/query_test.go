package marketplace

import (
	"fmt"
	"testing"
)

func TestAddCritera(t *testing.T) {
	q := NewQuery()
	c := Criteria{
		FilterType: FilterTypeExtensionID,
		Value:      "hepp",
	}
	q.AddCriteria(c)

	if len(q.Filters[0].Criteria) != 3 {
		t.Errorf("something went wrong, there should be 3 criteria, found %v", len(q.Filters[0].Criteria))
	}

	for i, c := range q.Filters[0].Criteria {
		switch i {
		case 0:
			if c.FilterType != FilterTypeTarget || c.Value != "Microsoft.VisualStudio.Code" {
				t.Errorf("first criteria was invalid, got: %v", c)
			}
		case 1:
			if c.FilterType != FilterTypeExtensionID || c.Value != "hepp" {
				t.Errorf("second criteria was invalid, got: %v", c)
			}
		case 2:
			if c.FilterType != 12 || c.Value != "4096" {
				t.Errorf("third criteria was invalid, got: %v", c)
			}
		default:
			t.Errorf("something went wrong i should be between 0 and 2")
		}
	}
}

func TestIsEmptyQuery(t *testing.T) {
	q := NewQuery()
	if !q.IsEmptyQuery() {
		t.Error("expected query to be empty")
	}
	q.AddCriteria(Criteria{FilterType: FilterTypeSearchText, Value: "test"})
	if q.IsEmptyQuery() {
		t.Error("expected query to NOT be empty")
	}
}

func TestIsValid(t *testing.T) {
	q := NewQuery()
	if !q.IsValid() {
		t.Error("expected query is not valid, it should be")
	}
	q = Query{}
	if q.IsValid() {
		t.Error("expected query is valid be it shoudn't be")
	}
}

func TestFlag(t *testing.T) {
	f := QueryFlag(950)
	fmt.Printf("%v - FlagExcludeNonValidated: %v\n", f, f.Is(FlagExcludeNonValidated))
	fmt.Printf("%v - FlagIncludeAssetURI: %v\n", f, f.Is(FlagIncludeAssetURI))
	fmt.Printf("%v - FlagIncludeCatergoryAndTags: %v\n", f, f.Is(FlagIncludeCatergoryAndTags))
	fmt.Printf("%v - FlagIncludeFiles: %v\n", f, f.Is(FlagIncludeFiles))
	fmt.Printf("%v - FlagIncludeInstallationTargets: %v\n", f, f.Is(FlagIncludeInstallationTargets))
	fmt.Printf("%v - FlagIncludeLatestVersionOnly: %v\n", f, f.Is(FlagIncludeLatestVersionOnly))
	fmt.Printf("%v - FlagIncludeSharedAccounts: %v\n", f, f.Is(FlagIncludeSharedAccounts))
	fmt.Printf("%v - FlagIncludeStatistics: %v\n", f, f.Is(FlagIncludeStatistics))
	fmt.Printf("%v - FlagIncludeVersionProperties: %v\n", f, f.Is(FlagIncludeVersionProperties))
	fmt.Printf("%v - FlagIncludeVersions: %v\n", f, f.Is(FlagIncludeVersions))
	fmt.Printf("%v - FlagUnpublished: %v\n", f, f.Is(FlagUnpublished))
}
