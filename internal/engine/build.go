package engine

import (
	"fmt"
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
				// We don't need to build anything. Just grab the existing layer.

			} else {
				fmt.Println("  -> [Cache] Result: [CACHE MISS] - Executing step...")
				cascadeMiss = true // Flip the switch! All steps below this will now miss.

				// TODO: We will eventually trigger Member 4 (Runtime) and Member 2 (Tar) here
				// to actually execute the command and save the new .tar file.
			}

			// 3. Update the timeline tracker so the NEXT step uses this new hash!
			currentDigest = key
		}
	}

	// 4. TODO: Write the final manifest to ~/.docksmith/images/

	fmt.Println("Build loop skeleton complete!")
	return nil
}
