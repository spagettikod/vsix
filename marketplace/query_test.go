package marketplace

import (
	"fmt"
	"testing"
)

func Test_Flags(t *testing.T) {
	i := 51

	fmt.Printf("FlagIncludeVersions = %v\n", FlagIncludeVersions.Is(QueryFlag(i)))
	fmt.Printf("FlagIncludeFiles = %v\n", FlagIncludeFiles.Is(QueryFlag(i)))
	fmt.Printf("FlagIncludeCatergoryAndTags = %v\n", FlagIncludeCatergoryAndTags.Is(QueryFlag(i)))
	fmt.Printf("FlagIncludeSharedAccounts = %v\n", FlagIncludeSharedAccounts.Is(QueryFlag(i)))
	fmt.Printf("FlagIncludeVersionProperties = %v\n", FlagIncludeVersionProperties.Is(QueryFlag(i)))
	fmt.Printf("FlagExcludeNonValidated = %v\n", FlagExcludeNonValidated.Is(QueryFlag(i)))
	fmt.Printf("FlagIncludeInstallationTargets = %v\n", FlagIncludeInstallationTargets.Is(QueryFlag(i)))
	fmt.Printf("FlagIncludeAssetURI = %v\n", FlagIncludeAssetURI.Is(QueryFlag(i)))
	fmt.Printf("FlagIncludeStatistics = %v\n", FlagIncludeStatistics.Is(QueryFlag(i)))
	fmt.Printf("FlagIncludeLatestVersionOnly = %v\n", FlagIncludeLatestVersionOnly.Is(QueryFlag(i)))
	fmt.Printf("FlagUnpublished = %v\n", FlagUnpublished.Is(QueryFlag(i)))

	i = 950
	fmt.Println("950")
	fmt.Printf("FlagIncludeVersions = %v\n", FlagIncludeVersions.Is(QueryFlag(i)))
	fmt.Printf("FlagIncludeFiles = %v\n", FlagIncludeFiles.Is(QueryFlag(i)))
	fmt.Printf("FlagIncludeCatergoryAndTags = %v\n", FlagIncludeCatergoryAndTags.Is(QueryFlag(i)))
	fmt.Printf("FlagIncludeSharedAccounts = %v\n", FlagIncludeSharedAccounts.Is(QueryFlag(i)))
	fmt.Printf("FlagIncludeVersionProperties = %v\n", FlagIncludeVersionProperties.Is(QueryFlag(i)))
	fmt.Printf("FlagExcludeNonValidated = %v\n", FlagExcludeNonValidated.Is(QueryFlag(i)))
	fmt.Printf("FlagIncludeInstallationTargets = %v\n", FlagIncludeInstallationTargets.Is(QueryFlag(i)))
	fmt.Printf("FlagIncludeAssetURI = %v\n", FlagIncludeAssetURI.Is(QueryFlag(i)))
	fmt.Printf("FlagIncludeStatistics = %v\n", FlagIncludeStatistics.Is(QueryFlag(i)))
	fmt.Printf("FlagIncludeLatestVersionOnly = %v\n", FlagIncludeLatestVersionOnly.Is(QueryFlag(i)))
	fmt.Printf("FlagUnpublished = %v\n", FlagUnpublished.Is(QueryFlag(i)))
}

func Test_AddCritera(t *testing.T) {
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
