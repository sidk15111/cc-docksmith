package engine

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// CreateLayerTar creates a deterministic tarball from a source directory.
func CreateLayerTar(sourceDir, destTarPath string) error {
	// 1. Gather all file paths first
	var paths []string
	err := filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		paths = append(paths, path)
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to walk directory: %v", err)
	}

	// 2. Sort lexicographically to guarantee consistent order
	sort.Strings(paths)

	// 3. Create the output tar file
	tarFile, err := os.Create(destTarPath)
	if err != nil {
		return fmt.Errorf("failed to create tar file: %v", err)
	}
	defer tarFile.Close()

	tw := tar.NewWriter(tarFile)
	defer tw.Close()

	// 4. Write entries
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			return err
		}

		header, err := tar.FileInfoHeader(info, info.Name())
		if err != nil {
			return err
		}

		// CRITICAL: Zero out timestamps so the file hash never fluctuates based on creation time
		var zeroTime time.Time
		header.ModTime = zeroTime
		header.AccessTime = zeroTime
		header.ChangeTime = zeroTime

		// Format the path to be relative to the source directory
		relPath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}
		if relPath == "." {
			continue // Skip the root directory itself
		}
		header.Name = filepath.ToSlash(relPath)

		if err := tw.WriteHeader(header); err != nil {
			return fmt.Errorf("failed to write tar header: %v", err)
		}

		// If it's a file, copy its contents into the tarball
		if !info.IsDir() {
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			if _, err := io.Copy(tw, file); err != nil {
				file.Close()
				return fmt.Errorf("failed to write file to tar: %v", err)
			}
			file.Close()
		}
	}

	return nil
}
