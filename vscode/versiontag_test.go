package vscode

import (
	"testing"
)

func TestParseVersionTag(t *testing.T) {
	type TestCase struct {
		StrTag     string
		Expected   VersionTag
		ShouldFail bool
	}
	tests := []TestCase{
		{StrTag: "foo.bar@:", ShouldFail: true},                 // missing version and target platform
		{StrTag: "foo.bar@", ShouldFail: true},                  // missing version
		{StrTag: "foo.bar:", ShouldFail: true},                  // missing version and target platform
		{StrTag: "foo.bar:darwin-arm64", ShouldFail: true},      // missing version
		{StrTag: "foo.bar@1.2.3:", ShouldFail: true},            // missing target platform
		{StrTag: "foobar", ShouldFail: true},                    // invalid uid
		{StrTag: "foobar@1.2.3:darwin-arm64", ShouldFail: true}, // invalid uid
		{StrTag: "foo.bar", Expected: VersionTag{UniqueID: UniqueID{"foo", "bar"}}, ShouldFail: false},
		{StrTag: "foo.bar@1.2.3", Expected: VersionTag{UniqueID: UniqueID{"foo", "bar"}, Version: "1.2.3"}, ShouldFail: false},
		{StrTag: "foo.bar@1.2.3:darwin-arm64", Expected: VersionTag{UniqueID: UniqueID{"foo", "bar"}, Version: "1.2.3", TargetPlatform: "darwin-arm64"}, ShouldFail: false},
	}

	for _, test := range tests {
		actual, err := ParseVersionTag(test.StrTag)
		if err != nil {
			if !test.ShouldFail {
				t.Fatalf("%s -> test case should not have failed: %s", test.StrTag, err)
			}
			continue // test was expected to fail, continue with next test case
		}

		if test.Expected != actual {
			t.Fatalf("%s -> expected %s but got %s", test.StrTag, test.Expected, actual)
		}
	}
}

func TestTargetVersionValidate(t *testing.T) {
	type TestCase struct {
		Tag      VersionTag
		Expected bool
	}

	tests := []TestCase{
		{Tag: VersionTag{UniqueID: UniqueID{Publisher: "", Name: "foo"}, Version: "", TargetPlatform: ""}, Expected: false},
		{Tag: VersionTag{UniqueID: UniqueID{Publisher: "foo", Name: ""}, Version: "", TargetPlatform: ""}, Expected: false},
		{Tag: VersionTag{UniqueID: UniqueID{Publisher: "foo", Name: "bar"}, Version: "", TargetPlatform: "darwin-arm64"}, Expected: false},
		{Tag: VersionTag{UniqueID: UniqueID{Publisher: "foo", Name: "bar"}, Version: "", TargetPlatform: ""}, Expected: true},
		{Tag: VersionTag{UniqueID: UniqueID{Publisher: "foo", Name: "bar"}, Version: "1.2.3", TargetPlatform: "darwin-arm64"}, Expected: true},
		{Tag: VersionTag{UniqueID: UniqueID{Publisher: "foo", Name: "bar"}, Version: "1.2.3", TargetPlatform: ""}, Expected: true},
	}

	for _, test := range tests {
		if test.Tag.Validate() != test.Expected {
			t.Fatalf("%s failed", test.Tag)
		}
	}
}
