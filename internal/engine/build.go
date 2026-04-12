package engine

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"docksmith/internal/cache"
	"docksmith/internal/parser"
)

// ImageManifest represents the final JSON structure stored in ~/.docksmith/images/
type ImageManifest struct {
	Name    string  `json:"name"`
	Tag     string  `json:"tag"`
	Digest  string  `json:"digest"`
	Created string  `json:"created"`
	Config  Config  `json:"config"`
	Layers  []Layer `json:"layers"`
}

type Config struct {
	Env        []string `json:"Env"`
	Cmd        []string `json:"Cmd"`
	WorkingDir string   `json:"WorkingDir"`
}

type Layer struct {
	Digest    string `json:"digest"`
	Size      int64  `json:"size"`
	CreatedBy string `json:"createdBy"`
}

// Build orchestrates the entire image creation process.
func Build(contextDir, imageName, tag string) error {
	docksmithFilePath := filepath.Join(contextDir, "Docksmithfile")

	// 1. Call Member 1's Parser
	instructions, err := parser.Parse(docksmithFilePath)
	if err != nil {
		return fmt.Errorf("parsing failed: %v", err)
	}

	// 2. Initialize the empty Manifest state
	manifest := ImageManifest{
		Name:    imageName,
		Tag:     tag,
		Created: time.Now().UTC().Format(time.RFC3339),
		Config: Config{
			WorkingDir: "/", // Default working directory
		},
	}

	// State trackers for the build loop
	// currentDigest tracks the hash of the last layer to feed into Member 3's Cache Master
	currentDigest := ""
	envMap := make(map[string]string)
	cascadeMiss := false
	//_ = currentDigest
	//_ = envMap

	fmt.Printf("Step 0: Starting build for %s:%s\n", imageName, tag)

	// 3. The Core Build Loop
	for i, inst := range instructions {
		fmt.Printf("Step %d/%d: %s %s\n", i+1, len(instructions), inst.Type, inst.Args)

		switch inst.Type {
		case "FROM":
			// TODO: Load base image layers
			currentDigest = "sha256:mock_base_digest"

		case "WORKDIR":
			// Set the working directory for subsequent instructions
			manifest.Config.WorkingDir = inst.Args
			fmt.Printf("  -> [Config] WorkingDir set to %s\n", inst.Args)

		case "ENV":
			// Parse KEY=VALUE
			parts := strings.SplitN(inst.Args, "=", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				val := strings.TrimSpace(parts[1])

				// Add to our live map for the Cache/Runtime
				envMap[key] = val

				// Append to the manifest config
				manifest.Config.Env = append(manifest.Config.Env, fmt.Sprintf("%s=%s", key, val))

				fmt.Printf("  -> [Config] Env %s set to %s\n", key, val)
			} else {
				return fmt.Errorf("invalid ENV format: %s", inst.Args)
			}

		case "CMD":
			// Store the CMD array
			manifest.Config.Cmd = []string{inst.Args}
			fmt.Printf("  -> [Config] Cmd set to %s\n", inst.Args)

		case "COPY", "RUN":
			// 1. Calculate the cache key
			key, err := cache.ComputeKey(currentDigest, inst.Original, manifest.Config.WorkingDir, envMap, nil)
			if err != nil {
				return fmt.Errorf("failed to compute cache key: %v", err)
			}

			fmt.Printf("  -> [Cache] Computed Key: %s\n", key)

			// 2. Check the cache AND the cascade flag
			if !cascadeMiss && cache.IsHit(key) {
				fmt.Println("  -> [Cache] Result: [CACHE HIT] - Reusing layer!")

				// Get the file size for the manifest
				homeDir, _ := os.UserHomeDir()
				hashOnly := strings.TrimPrefix(key, "sha256:")
				layerPath := filepath.Join(homeDir, ".docksmith", "layers", hashOnly+".tar")
				info, _ := os.Stat(layerPath)

				manifest.Layers = append(manifest.Layers, Layer{
					Digest:    key,
					Size:      info.Size(),
					CreatedBy: inst.Original,
				})

			} else {
				fmt.Println("  -> [Cache] Result: [CACHE MISS] - Executing step...")
				cascadeMiss = true

				// --- THE EXECUTION ENGINE ---

				// A. Create a temporary folder for this step's output
				tempDir, err := os.MkdirTemp("", "docksmith-layer-*")
				if err != nil {
					return fmt.Errorf("failed to create temp dir: %v", err)
				}

				// B. Mock the execution (Writing dummy files to represent the work)
				if inst.Type == "COPY" {
					os.WriteFile(filepath.Join(tempDir, "copied_data.txt"), []byte("mock file from context"), 0644)
				} else if inst.Type == "RUN" {
					os.WriteFile(filepath.Join(tempDir, "run_output.txt"), []byte("mock output from command"), 0644)
				}

				// C. Package the result into a new Tar Layer!
				homeDir, _ := os.UserHomeDir()
				hashOnly := strings.TrimPrefix(key, "sha256:")
				layerDest := filepath.Join(homeDir, ".docksmith", "layers", hashOnly+".tar")

				fmt.Printf("    -> [Tar] Compressing layer to %s.tar\n", hashOnly[:12])
				if err := CreateLayerTar(tempDir, layerDest); err != nil {
					return fmt.Errorf("failed to create layer tar: %v", err)
				}

				// Clean up the temp folder
				os.RemoveAll(tempDir)

				// D. Add the new layer to the Manifest
				info, _ := os.Stat(layerDest)
				manifest.Layers = append(manifest.Layers, Layer{
					Digest:    key,
					Size:      info.Size(),
					CreatedBy: inst.Original,
				})
			}

			// 3. Update the timeline tracker so the NEXT step uses this new hash
			currentDigest = key
		}
	}

	// 4. TODO: Write the final manifest to ~/.docksmith/images/
	// 4. Write the final manifest to ~/.docksmith/images/
	fmt.Println("Step 7: Finalizing image manifest...")

	// A. The PDF Rule: Digest must be empty when calculating the hash
	manifest.Digest = ""

	// Convert the Go struct to beautifully formatted JSON
	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode manifest: %v", err)
	}

	// B. Calculate the SHA-256 hash of the JSON bytes
	h := sha256.New()
	h.Write(manifestBytes)
	manifestHash := hex.EncodeToString(h.Sum(nil))

	// C. Update the manifest with its true digest
	manifest.Digest = "sha256:" + manifestHash

	// D. Write it to disk!
	// The PDF requires the file to be stored in images/ named by the image name.
	// For simplicity, we will save it as <name>_<tag>.json
	homeDir, _ := os.UserHomeDir()
	manifestName := fmt.Sprintf("%s_%s.json", manifest.Name, manifest.Tag)
	manifestPath := filepath.Join(homeDir, ".docksmith", "images", manifestName)

	// Re-marshal to include the new digest field
	finalBytes, _ := json.MarshalIndent(manifest, "", "  ")

	if err := os.WriteFile(manifestPath, finalBytes, 0644); err != nil {
		return fmt.Errorf("failed to write manifest to disk: %v", err)
	}

	fmt.Printf("Successfully built %s %s:%s\n", manifest.Digest[:19], manifest.Name, manifest.Tag)

	fmt.Println("Build loop skeleton complete!")
	return nil
}
