package main

import (
	"fmt"
	"os"
	"path/filepath"
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
	// 1. Initialize state directories before doing anything else
	if err := initDocksmithDirs(); err != nil {
		fmt.Fprintf(os.Stderr, "Fatal error initializing state: %v\n", err)
		os.Exit(1)
	}

	// 2. Basic CLI Routing
	// os.Args[0] is the program name itself. We need at least one command (os.Args[1]).
	if len(os.Args) < 2 {
		fmt.Println("Usage: docksmith <command> [args]")
		fmt.Println("Commands: build, run, images, rmi")
		os.Exit(1)
	}

	command := os.Args[1]

	// Route the command
	switch command {
	case "build":
		// Eventually this will call something like engine.Build(os.Args[2:])
		fmt.Println("[Core] Routing to Build Engine...")
	case "run":
		fmt.Println("[Core] Routing to Runtime Isolation...")
	case "images":
		fmt.Println("[Core] Listing images...")
	case "rmi":
		fmt.Println("[Core] Removing image...")
	default:
		fmt.Printf("docksmith: '%s' is not a docksmith command.\n", command)
		os.Exit(1)
	}
}
