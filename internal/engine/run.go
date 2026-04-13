package engine

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// RunImage boots a container from a compiled image manifest
func RunImage(imageName, tag string, userCmd []string) error {
	homeDir, _ := os.UserHomeDir()
	manifestName := fmt.Sprintf("%s_%s.json", imageName, tag)
	manifestPath := filepath.Join(homeDir, ".docksmith", "images", manifestName)

	// 1. Read the saved Manifest
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("image %s:%s not found. Did you build it?", imageName, tag)
	}

	var manifest ImageManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return fmt.Errorf("failed to parse manifest: %v", err)
	}

	// 2. Setup Container Filesystem
	fmt.Println("=> Assembling container filesystem...")
	tempDir, err := os.MkdirTemp("", "docksmith-run-*")
	if err != nil {
		return err
	}

	// CRITICAL: We use 'defer' so that even if the container crashes,
	// Go will automatically unmount and delete the temporary folders before exiting!
	defer func() {
		exec.Command("sudo", "umount", filepath.Join(tempDir, "merged")).Run()
		exec.Command("sudo", "rm", "-rf", tempDir).Run()
	}()

	baseDir := filepath.Join(tempDir, "base")
	upperDir := filepath.Join(tempDir, "upper")
	workDir := filepath.Join(tempDir, "work")
	mergedDir := filepath.Join(tempDir, "merged")
	os.MkdirAll(baseDir, 0755)
	os.MkdirAll(upperDir, 0755)
	os.MkdirAll(workDir, 0755)
	os.MkdirAll(mergedDir, 0755)

	// Unpack all Layers
	for _, layer := range manifest.Layers {
		hashOnly := strings.TrimPrefix(layer.Digest, "sha256:")
		layerTar := filepath.Join(homeDir, ".docksmith", "layers", hashOnly+".tar")
		exec.Command("tar", "-xf", layerTar, "-C", baseDir).Run()
	}

	// Mount OverlayFS
	mountOpts := fmt.Sprintf("lowerdir=%s,upperdir=%s,workdir=%s", baseDir, upperDir, workDir)
	if err := exec.Command("sudo", "mount", "-t", "overlay", "overlay", "-o", mountOpts, mergedDir).Run(); err != nil {
		return fmt.Errorf("failed to mount overlayfs: %v", err)
	}

	// 3. Determine Command to Run
	var finalCmd []string
	if len(userCmd) > 0 {
		finalCmd = userCmd
	} else if len(manifest.Config.Cmd) > 0 {
		// manifest.Config.Cmd is already []string, so this is correct
		finalCmd = manifest.Config.Cmd
	} else {
		finalCmd = []string{"/bin/sh"}
	}

	// BUG FIX: If manifest.Config.Cmd was parsed strangely from JSON
	// and contains a single string like `["/bin/sh"]`, let's clean it up.
	if len(finalCmd) == 1 {
		clean := strings.Trim(finalCmd[0], "[]\"")
		finalCmd[0] = clean
	}

	// 4. Trigger Member 4's Runtime (Interactive Mode!)
	fmt.Printf("=> Booting isolated container: %s\n\n", strings.Join(finalCmd, " "))

	// Get the absolute path to the currently running docksmith binary
	docksmithExe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to locate docksmith executable: %v", err)
	}

	// Trigger Member 4's Runtime (Interactive Mode!)
	fmt.Printf("=> Booting isolated container: %s\n\n", strings.Join(finalCmd, " "))

	runArgs := []string{"-E", docksmithExe, "child", mergedDir, manifest.Config.WorkingDir}
	runArgs = append(runArgs, finalCmd...)

	runCmd := exec.Command("sudo", runArgs...)

	// THE MAGIC: Plugging your keyboard directly into the container
	runCmd.Stdin = os.Stdin
	runCmd.Stdout = os.Stdout
	runCmd.Stderr = os.Stderr

	// Inject Environment Variables from the image (e.g., DEBUG=true)
	runCmd.Env = os.Environ()
	runCmd.Env = append(runCmd.Env, manifest.Config.Env...)

	runCmd.Env = append(runCmd.Env, "PATH=/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin")
	// Run waits for you to type "exit" inside the container
	if err := runCmd.Run(); err != nil {
		return fmt.Errorf("container exited: %v", err)
	}

	fmt.Println("\n=> Container shutdown gracefully.")
	return nil
}
