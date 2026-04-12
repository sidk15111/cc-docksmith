package engine

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// RemoveImage deletes the manifest and its associated layers
func RemoveImage(imageName, tag string) error {
	homeDir, _ := os.UserHomeDir()
	manifestName := fmt.Sprintf("%s_%s.json", imageName, tag)
	manifestPath := filepath.Join(homeDir, ".docksmith", "images", manifestName)

	// 1. Read the manifest so we know which layers to delete
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("image %s:%s not found", imageName, tag)
	}

	var manifest ImageManifest
	json.Unmarshal(data, &manifest)

	// 2. Delete the manifest file itself
	if err := os.Remove(manifestPath); err != nil {
		return fmt.Errorf("failed to remove manifest: %v", err)
	}

	// 3. Delete the layers
	for _, layer := range manifest.Layers {
		hashOnly := strings.TrimPrefix(layer.Digest, "sha256:")
		layerPath := filepath.Join(homeDir, ".docksmith", "layers", hashOnly+".tar")
		
		// Note: In a production engine, we'd check if another image 
		// still needs this layer. For Docksmith, we'll just try to delete.
		os.Remove(layerPath)
	}

	fmt.Printf("Untagged: %s:%s\n", imageName, tag)
	fmt.Printf("Deleted: %s\n", manifest.Digest)
	return nil
}
