package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"docksmith/internal/cache"  //mem3 cache testing
	"docksmith/internal/engine" //this was for mem2 testing test-tar

	//"docksmith/internal/parser"  // mem1 parser testing
	"docksmith/internal/runtime" //mem4 isolation and runtime
)

// initDocksmithDirs creates the required ~/.docksmith structure
func initDocksmithDirs() error {
	// Dynamically find the home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("could not find home directory: %v", err)
	}

	baseDir := filepath.Join(homeDir, ".docksmith")

	// Define the required subdirectories
	dirs := []string{
		filepath.Join(baseDir, "images"),
		filepath.Join(baseDir, "layers"),
		filepath.Join(baseDir, "cache"),
	}

	// Create each directory (MkdirAll acts like `mkdir -p`, it won't fail if the dir already exists)
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %v", d, err)
		}
	}

	return nil
}

func main() {
	// 1. Bounds check FIRST. Prevent panic if user types no arguments.
	if len(os.Args) < 2 {
		fmt.Println("Usage: docksmith <command> [args]")
		fmt.Println("Commands: build, run, images, rmi")
		os.Exit(1)
	}

	// 2. Safely grab the command
	command := os.Args[1]

	// 3. Initialize state directories ONLY if this is NOT the hidden child process
	if command != "child" {
		if err := initDocksmithDirs(); err != nil {
			fmt.Fprintf(os.Stderr, "Fatal error initializing state: %v\n", err)
			os.Exit(1)
		}
	}

	homeDir, _ := os.UserHomeDir() // Used for your test-tar command

	// Route the command
	switch command {

	case "build":
		// Usage: docksmith build -t myapp:latest .
		if len(os.Args) < 5 || os.Args[2] != "-t" {
			fmt.Println("Usage: docksmith build -t <name:tag> <context>")
			os.Exit(1)
		}

		fullTag := os.Args[3]
		contextDir := os.Args[4]

		// Parse the fullTag dynamically
		parts := strings.Split(fullTag, ":")
		name := parts[0]
		tag := "latest" // Default to latest if no tag is provided
		if len(parts) > 1 {
			tag = parts[1]
		}

		if err := engine.Build(contextDir, name, tag); err != nil {
			fmt.Fprintf(os.Stderr, "Build failed: %v\n", err)
			os.Exit(1)
		}

	case "run":
		// Usage: docksmith run myapp:latest [/bin/sh]
		if len(os.Args) < 3 {
			fmt.Println("Usage: docksmith run <image:tag> [command...]")
			os.Exit(1)
		}

		fullTag := os.Args[2]
		parts := strings.Split(fullTag, ":")
		name := parts[0]
		tag := "latest"
		if len(parts) > 1 {
			tag = parts[1]
		}

		// Grab any extra arguments as the override command
		var userCmd []string
		if len(os.Args) > 3 {
			userCmd = os.Args[3:]
		}

		if err := engine.RunImage(name, tag, userCmd); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}

	case "images":
		if err := engine.ListImages(); err != nil {
			fmt.Fprintf(os.Stderr, "Error listing images: %v\n", err)
			os.Exit(1)
		}

	case "rmi":
		if len(os.Args) < 3 {
			fmt.Println("Usage: docksmith rmi <image:tag>")
			os.Exit(1)
		}
		fullTag := os.Args[2]
		parts := strings.Split(fullTag, ":")
		name := parts[0]
		tag := "latest"
		if len(parts) > 1 {
			tag = parts[1]
		}

		if err := engine.RemoveImage(name, tag); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "test-tar":
		if len(os.Args) < 3 {
			fmt.Println("Usage: docksmith test-tar <source_directory>")
			os.Exit(1)
		}
		sourceDir := os.Args[2]
		destTar := filepath.Join(filepath.Join(homeDir, ".docksmith", "layers"), "test_layer.tar")

		fmt.Printf("Packing %s into %s...\n", sourceDir, destTar)
		if err := engine.CreateLayerTar(sourceDir, destTar); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Tar created successfully! Now check its hash.")

	case "test-cache":
		// Mock state representing a build step
		prevDigest := "sha256:1234567890abcdef"
		instruction := "RUN echo 'Building Docksmith'"
		workdir := "/app"
		env := map[string]string{
			"PATH":  "/usr/bin",
			"DEBUG": "true",
		}

		// 1. Compute the key the first time
		key1, _ := cache.ComputeKey(prevDigest, instruction, workdir, env, nil)
		fmt.Println("Hash 1 (Initial):     ", key1)

		// 2. Compute it again without changing anything
		key2, _ := cache.ComputeKey(prevDigest, instruction, workdir, env, nil)
		fmt.Println("Hash 2 (No Changes):  ", key2)

		// 3. Modify the ENV slightly to simulate a changed context
		env["DEBUG"] = "false"
		key3, _ := cache.ComputeKey(prevDigest, instruction, workdir, env, nil)
		fmt.Println("Hash 3 (Modified ENV):", key3)

		// Verify the logic
		if key1 == key2 && key1 != key3 {
			fmt.Println("\nSuccess! Cache logic is deterministic and correctly detects changes.")
		} else {
			fmt.Println("\nFailure: Cache keys did not behave as expected.")
		}
	case "child":
		// This is hidden from the user. docksmith calls this internally.
		if err := runtime.Child(); err != nil {
			fmt.Fprintf(os.Stderr, "Container error: %v\n", err)
			os.Exit(1)
		}

	case "test-run":
		// Usage: sudo docksmith test-run <rootfs_path> <command>
		if len(os.Args) < 4 {
			fmt.Println("Usage: docksmith test-run <rootfs_path> <command>")
			os.Exit(1)
		}
		rootfs := os.Args[2]
		cmdArgs := os.Args[3:]

		fmt.Printf("Starting isolated container in %s...\n", rootfs)

		// Setup mock environment and workdir
		env := []string{"MOCK_ENV=docksmith_test"}
		workdir := "/"

		if err := runtime.Run(rootfs, workdir, env, cmdArgs); err != nil {
			fmt.Printf("Runtime failed: %v\n", err)
			os.Exit(1)
		}

	default:
		fmt.Printf("docksmith: '%s' is not a docksmith command.\n", command)
		os.Exit(1)
	}
}
