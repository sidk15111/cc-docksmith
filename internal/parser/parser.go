package parser

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// Instruction represents a single parsed line from the Docksmithfile
type Instruction struct {
	Type     string // e.g., "FROM", "RUN"
	Args     string // e.g., "alpine:3.18", "echo 'hello'"
	Original string // The exact raw string (needed for cache hashing later)
	LineNum  int    // For error reporting
}

// Parse reads a Docksmithfile and returns a slice of Instructions
func Parse(filePath string) ([]Instruction, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open Docksmithfile: %v", err)
	}
	defer file.Close()

	var instructions []Instruction
	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		rawLine := scanner.Text()

		// Trim leading/trailing whitespace
		trimmedLine := strings.TrimSpace(rawLine)

		// Skip empty lines (and optionally comments if you want to support them)
		if trimmedLine == "" || strings.HasPrefix(trimmedLine, "#") {
			continue
		}

		// Split into the Instruction (e.g., "RUN") and the Arguments
		parts := strings.SplitN(trimmedLine, " ", 2)
		cmdType := strings.ToUpper(parts[0])

		args := ""
		if len(parts) > 1 {
			args = strings.TrimSpace(parts[1])
		}

		// Validate against the strict 6 allowed commands
		switch cmdType {
		case "FROM", "COPY", "RUN", "WORKDIR", "ENV", "CMD":
			instructions = append(instructions, Instruction{
				Type:     cmdType,
				Args:     args,
				Original: rawLine, // Keep the raw line for the cache digest later
				LineNum:  lineNum,
			})
		default:
			// Hard failure on unrecognized commands as per requirements
			return nil, fmt.Errorf("syntax error on line %d: unrecognized instruction '%s'", lineNum, cmdType)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading file: %v", err)
	}

	return instructions, nil
}
