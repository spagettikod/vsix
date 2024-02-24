package database

import (
	"fmt"
	"io/fs"
	"os"
	"path"
)

type DirEntryProcessor func(fs fs.FS, root string, dirEntry fs.DirEntry) (PruneResult, error)

type PruneResult struct {
	Kept     []string
	Removed  []string
	Optional []string
}

func NewPruneResult() PruneResult {
	return PruneResult{[]string{}, []string{}, []string{}}
}

// CleanDBFiles walks the database directory tree starting at root. It returns a list of
// files to keep, delete and an error if an error occurs while processing. File paths in
// the lists are start at the given database directory and are not absolute.
// It does not check if the files in the version metadata are there or if there are more files
// in the lowest folder than in the metadata file.
func CleanDBFiles(root string) (PruneResult, error) {
	return process(os.DirFS("."), root, rootProcessor)
}

func process(fsys fs.FS, dir string, processor DirEntryProcessor) (PruneResult, error) {
	result := NewPruneResult()
	files, err := fs.ReadDir(fsys, dir)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	for _, file := range files {
		fullPath := path.Join(dir, file.Name())
		var dirResult PruneResult
		dirResult, err = processor(fsys, fullPath, file)
		if err != nil {
			return result, err
		}
		result.Kept = append(result.Kept, dirResult.Kept...)
		result.Removed = append(result.Removed, dirResult.Removed...)
		result.Optional = append(result.Optional, dirResult.Optional...)
	}
	return result, nil
}

// noMetaFileProcessor a common processor used by the root, publisher and version directories which share the same logic.
func noMetaFileProcessor(fsys fs.FS, fullPath string, entry fs.DirEntry, next DirEntryProcessor) (PruneResult, error) {
	result := NewPruneResult()
	if entry.IsDir() {
		var err error
		result, err = process(fsys, fullPath, next)
		if err != nil {
			return result, err
		}
		if len(result.Kept) == 0 {
			result.Removed = append(result.Removed, fullPath)
		} else {
			result.Kept = append(result.Kept, fullPath)
		}
	} else {
		result.Removed = append(result.Removed, fullPath)
	}
	return result, nil
}

// rootProcessor the prune logic used for files in the root folder. Remove everything that isn't an empty subfolder.
func rootProcessor(fsys fs.FS, fullPath string, entry fs.DirEntry) (PruneResult, error) {
	return noMetaFileProcessor(fsys, fullPath, entry, publisherProcessor)
}

// publisherProcessor the prune logic used for files in the publisher folder. Remove everything that isn't an empty subfolder.
func publisherProcessor(fsys fs.FS, fullPath string, entry fs.DirEntry) (PruneResult, error) {
	// kept, removed, optional, err = noMetaFileProcessor(fullPath, entry, extensionProcessor)
	result := NewPruneResult()
	if entry.IsDir() {
		var err error
		result, err = process(fsys, fullPath, extensionProcessor)
		if err != nil {
			return result, err
		}
		if len(result.Kept) == 0 {
			result.Removed = append(result.Removed, fullPath)
		} else {
			if len(result.Kept) == 1 {
				result.Optional = append(result.Optional, fullPath)
			} else {
				result.Kept = append(result.Kept, fullPath)
			}
		}
	} else {
		result.Removed = append(result.Removed, fullPath)
	}
	return result, nil
}

// extensionProcessor the prune logic used for files in the extension folder. Remove everything but the extension metadata file and non empty subfolders.
func extensionProcessor(fsys fs.FS, fullPath string, entry fs.DirEntry) (PruneResult, error) {
	result := NewPruneResult()
	if entry.IsDir() {
		var err error
		result, err = process(fsys, fullPath, versionProcessor)
		if err != nil {
			return result, err
		}
		if len(result.Kept) == 0 {
			// optional = append(optional, fullPath)
			result.Removed = append(result.Removed, fullPath)
		} else {
			result.Kept = append(result.Kept, fullPath)
		}
	} else {
		if entry.Name() == extensionMetadataFileName {
			result.Kept = append(result.Kept, fullPath)
		} else {
			result.Removed = append(result.Removed, fullPath)
		}
	}
	return result, nil
}

// versionProcessor the prune logic used for files in the version folder. Remove everything that isn't an empty subfolder.
func versionProcessor(fsys fs.FS, fullPath string, entry fs.DirEntry) (PruneResult, error) {
	return noMetaFileProcessor(fsys, fullPath, entry, versionIdProcessor)
}

// versionIdProcessor the prune logic used for files in the version ID folder. Remove subfolders and keep all files.
// TODO should we remove version ids if there is only the metadata file?
func versionIdProcessor(fsys fs.FS, fullPath string, entry fs.DirEntry) (PruneResult, error) {
	result := NewPruneResult()
	if entry.IsDir() {
		result.Removed = append(result.Removed, fullPath)
	} else {
		result.Kept = append(result.Kept, fullPath)
	}
	return result, nil
}
