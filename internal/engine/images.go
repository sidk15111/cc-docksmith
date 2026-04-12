package engine

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
)

// ListImages scans the images directory and prints a formatted table
func ListImages() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %v", err)
	}

	imagesDir := filepath.Join(homeDir, ".docksmith", "images")
	
	// Read everything inside the images directory
	entries, err := os.ReadDir(imagesDir)
	if err != nil {
		if os.IsNotExist(err) {
			// If the folder doesn't exist yet, just print the empty header
			fmt.Println("REPOSITORY\tTAG\tIMAGE ID\tCREATED")
			return nil
		}
		return err
	}

	// Initialize the TabWriter for perfectly aligned columns
	// The parameters (minwidth, tabwidth, padding, padchar, flags) control the spacing
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 4, ' ', 0)
	
	// Print the Docker-style header
	fmt.Fprintln(w, "REPOSITORY\tTAG\tIMAGE ID\tCREATED")

	for _, entry := range entries {
		// We only care about .json manifest files
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		manifestPath := filepath.Join(imagesDir, entry.Name())
		data, err := os.ReadFile(manifestPath)
		if err != nil {
			continue 
		}

		// Parse the JSON back into our struct
		var manifest ImageManifest
		if err := json.Unmarshal(data, &manifest); err != nil {
			continue 
		}

		// Format the Image ID (Strip "sha256:" and grab just the first 12 characters)
		shortID := strings.TrimPrefix(manifest.Digest, "sha256:")
		if len(shortID) > 12 {
			shortID = shortID[:12]
		}

		// Print the row to the tabwriter
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", manifest.Name, manifest.Tag, shortID, manifest.Created)
	}

	// Flush forces the tabwriter to calculate the spacing and print to the terminal
	w.Flush()
	return nil
}
