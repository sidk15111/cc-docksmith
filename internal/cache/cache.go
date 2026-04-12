package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"sort"
)

// ComputeKey calculates the deterministic SHA-256 cache key for a build step.
func ComputeKey(prevDigest, instruction, workdir string, env map[string]string, copySourcePaths []string) (string, error) {
	h := sha256.New()

	// 1. Previous Layer Digest (ensures a changed base image invalidates everything downstream)
	h.Write([]byte(prevDigest + "\n"))

	// 2. Full instruction text (e.g., "RUN apt-get update")
	h.Write([]byte(instruction + "\n"))

	// 3. Current WORKDIR state
	h.Write([]byte(workdir + "\n"))

	// 4. Current ENV state (MUST be lexicographically sorted by key to guarantee identical hashes)
	var envKeys []string
	for k := range env {
		envKeys = append(envKeys, k)
	}
	sort.Strings(envKeys)

	for _, k := range envKeys {
		h.Write([]byte(fmt.Sprintf("%s=%s\n", k, env[k])))
	}

	// 5. COPY instructions only: Hash the actual file contents, sorted by path
	if len(copySourcePaths) > 0 {
		sort.Strings(copySourcePaths)
		for _, path := range copySourcePaths {
			fileHash, err := hashFileBytes(path)
			if err != nil {
				return "", fmt.Errorf("failed to hash file %s: %v", path, err)
			}
			h.Write([]byte(fileHash + "\n"))
		}
	}

	// Return the final formatted digest
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}

// hashFileBytes is a helper function that streams a file's raw bytes through SHA-256
func hashFileBytes(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	h := sha256.New()
	if _, err := io.Copy(h, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
