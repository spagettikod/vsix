package database

import (
	"testing"

	"github.com/spagettikod/vsix/marketplace"
	"github.com/spagettikod/vsix/vscode"
)

var (
	memdb          *DB
	testExtensions = []marketplace.ExtensionRequest{
		{UniqueID: "golang.Go", Version: "0.31.1"},
		{UniqueID: "esbenp.prettier-vscode", Version: "9.3.0"},
		{UniqueID: "esbenp.prettier-vscode", Version: "9.2.0"},
		{UniqueID: "__no_real_extension", Version: "0.0.0"},
		{UniqueID: "ms-vscode-remote.remote-ssh", Version: "0.77.2022030315", PreRelease: true}, // pre-release
	}
	expectedExtensionCount        = 3
	expectedExtensionVersionCount = 4
)

func getTestExtensionRequest(uniqueID string) (marketplace.ExtensionRequest, bool) {
	for _, te := range testExtensions {
		if te.UniqueID == uniqueID {
			return te, true
		}
	}
	return marketplace.ExtensionRequest{}, false
}

func isTestExtension(ext vscode.Extension, version string) bool {
	for _, te := range testExtensions {
		if te.UniqueID == ext.UniqueID() && te.Version == version {
			return true
		}
	}
	return false
}

func TestIntegrationTests(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration tests when -test.short")
	}

	var err error
	memdb, err = OpenMem()
	if err != nil {
		t.Fatal(err)
	}

	// download extensions to test database
	for _, pe := range testExtensions {
		extension, err := pe.Download(pe.PreRelease)
		if err != nil {
			if pe.UniqueID == "__no_real_extension" {
				continue
			}
			t.Fatal(err)
		}
		if err := memdb.SaveExtensionMetadata(extension); err != nil {
			t.Fatal(err)
		}
		for _, v := range extension.Versions {
			if !pe.ValidTargetPlatform(v) {
				continue
			}
			if memdb.VersionExists(extension.UniqueID(), v) {
				continue
			}
			if err := memdb.SaveVersionMetadata(extension, v); err != nil {
				t.Fatal(err)
			}
			for _, a := range v.Files {
				b, err := a.Download()
				if err != nil {
					t.Fatal(err)
				}
				if err := memdb.SaveAssetFile(extension, v, a, b); err != nil {
					t.Fatal(err)
				}
			}
		}
	}

	// reload database with downloaded files
	if err := memdb.Reload(); err != nil {
		t.Fatal(err)
	}

	t.Run("TestValidateExtensionDB", TestValidateExtensionDB)
	t.Run("TestDump", TestDump)
	t.Run("TestFindByUniqueID", TestFindByUniqueID)
	t.Run("TestFindByExtensionID", TestFindByExtensionID)
	t.Run("TestVersionExists", TestVersionExists)
	t.Run("TestEmpty", TestEmpty)
	t.Run("TestSearch", TestSearch)
}

func TestValidateExtensionDB(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration tests when -test.short")
	}
	exts := memdb.List()
	if len(exts) != expectedExtensionCount {
		t.Errorf("expected %v extensions, got %v", expectedExtensionCount, len(exts))
	}
	versionCount := 0
	for _, e := range exts {
		versionCount += len(e.Versions)
	}
	if versionCount != expectedExtensionVersionCount {
		t.Errorf("expected %v extensions versions, got %v", expectedExtensionVersionCount, versionCount)
	}
}

func TestDump(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration tests when -test.short")
	}
	exts := memdb.List()
	for _, ext := range exts {
		if testReq, found := getTestExtensionRequest(ext.UniqueID()); found {
			if !isTestExtension(ext, ext.LatestVersion(testReq.PreRelease)) {
				t.Errorf("found %v %v, which is not part of expected extensions", ext.UniqueID(), ext.LatestVersion(testReq.PreRelease))
			}
		} else {
			t.Errorf("could not find extension %v, among extensions in the test", ext.UniqueID())
		}
	}
}

func TestFindByUniqueID(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration tests when -test.short")
	}
	e := memdb.FindByUniqueID(false, "esbenp.prettier-vscode")
	if len(e) != 1 {
		t.Fatalf("extected %v extensions, got %v", 1, len(e))
	}

	if len(e[0].Versions) != 2 {
		t.Errorf("extected %v extension versions, got %v", 2, len(e[0].Versions))
	}

	if testReq, found := getTestExtensionRequest(e[0].UniqueID()); found {
		if e[0].LatestVersion(testReq.PreRelease) != "9.3.0" {
			t.Errorf("extected extension version %v, got %v", "9.3.0", e[0].LatestVersion(testReq.PreRelease))
		}
	} else {
		t.Errorf("could not find extension %v, among extensions in the test", e[0].UniqueID())
	}

	e = memdb.FindByUniqueID(true, "esbenp.prettier-vscode")
	if len(e) != 1 {
		t.Fatalf("extected %v extensions, got %v", 1, len(e))
	}

	if len(e[0].Versions) != 1 {
		t.Errorf("extected %v extension versions, got %v", 1, len(e[0].Versions))
	}

	if e[0].Versions[0].Version != "9.3.0" {
		t.Errorf("extected extension version %v, got %v", "9.3.0", e[0].Versions[0].Version)
	}
}

func TestFindByExtensionID(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration tests when -test.short")
	}
	e := memdb.FindByExtensionID(false, "d6f6cfea-4b6f-41f4-b571-6ad2ab7918da")
	if len(e) != 1 {
		t.Fatalf("extected %v extensions, got %v", 1, len(e))
	}

	if len(e[0].Versions) != 1 {
		t.Errorf("extected %v extension versions, got %v", 1, len(e[0].Versions))
	}

	if testReq, found := getTestExtensionRequest(e[0].UniqueID()); found {
		if e[0].LatestVersion(testReq.PreRelease) != "0.31.1" {
			t.Errorf("extected extension version %v, got %v", "0.31.1", e[0].LatestVersion(testReq.PreRelease))
		}
	} else {
		t.Errorf("could not find extension %v, among extensions in the test", e[0].UniqueID())
	}

	if e[0].UniqueID() != "golang.Go" {
		t.Errorf("extected extension UniqueId %v, got %v", "golang.Go", e[0].UniqueID())
	}

	e = memdb.FindByExtensionID(true, "d6f6cfea-4b6f-41f4-b571-6ad2ab7918da")
	if len(e) != 1 {
		t.Fatalf("extected %v extensions, got %v", 1, len(e))
	}

	if len(e[0].Versions) != 1 {
		t.Errorf("extected %v extension versions, got %v", 1, len(e[0].Versions))
	}

	if testReq, found := getTestExtensionRequest(e[0].UniqueID()); found {
		if e[0].LatestVersion(testReq.PreRelease) != "0.31.1" {
			t.Errorf("extected extension version %v, got %v", "0.31.1", e[0].LatestVersion(testReq.PreRelease))
		}
	} else {
		t.Errorf("could not find extension %v, among extensions in the test", e[0].UniqueID())
	}

	if e[0].UniqueID() != "golang.Go" {
		t.Errorf("extected extension UniqueId %v, got %v", "golang.Go", e[0].UniqueID())
	}
}

func TestVersionExists(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration tests when -test.short")
	}
	v := vscode.Version{Version: "9.3.0", AssetURI: "/esbenp/prettier-vscode/9.3.0/1645467140557"}
	if !memdb.VersionExists("esbenp.prettier-vscode", v) {
		t.Errorf("extected version %v to exist but it was not found", "9.3.0")
	}
	v.Version = "x.y.z"
	if memdb.VersionExists("esbenp.prettier-vscode", v) {
		t.Errorf("didn't extect to find version %v", "x.y.z")
	}
}

func TestEmpty(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration tests when -test.short")
	}
	if memdb.Empty() {
		t.Error("expected database to contain values instead Empty returned true")
	}
}

func TestSearch(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration tests when -test.short")
	}
	e := memdb.Search(false, "Code formatter")
	if len(e) != 1 {
		t.Fatalf("extected %v extensions, got %v", 1, len(e))
	}

	if len(e[0].Versions) != 2 {
		t.Errorf("extected %v extension versions, got %v", 2, len(e[0].Versions))
	}

	if testReq, found := getTestExtensionRequest(e[0].UniqueID()); found {
		if e[0].LatestVersion(testReq.PreRelease) != "9.3.0" {
			t.Errorf("extected extension version %v, got %v", "9.3.0", e[0].LatestVersion(testReq.PreRelease))
		}
	} else {
		t.Errorf("could not find extension %v, among extensions in the test", e[0].UniqueID())
	}

	e = memdb.Search(true, "Code formatter")
	if len(e) != 1 {
		t.Fatalf("extected %v extensions, got %v", 1, len(e))
	}

	if len(e[0].Versions) != 1 {
		t.Errorf("extected %v extension versions, got %v", 1, len(e[0].Versions))
	}

	if e[0].Versions[0].Version != "9.3.0" {
		t.Errorf("extected extension version %v, got %v", "9.3.0", e[0].Versions[0].Version)
	}
}
