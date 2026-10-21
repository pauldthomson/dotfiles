package main

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// Read only this line: buffering beyond the newline would discard subsequent
// confirmations when stdin is a pipe or a test fixture.
func promptLine(prompt string) (string, error) {
	fmt.Print(prompt)
	var line strings.Builder
	var b [1]byte
	for {
		n, err := os.Stdin.Read(b[:])
		if n > 0 {
			if b[0] == '\n' {
				return strings.TrimSuffix(line.String(), "\r"), nil
			}
			line.WriteByte(b[0])
		}
		if err != nil {
			if err == io.EOF && line.Len() > 0 {
				return strings.TrimSuffix(line.String(), "\r"), nil
			}
			return "", err
		}
	}
}
func confirm(prompt string) (bool, error) {
	line, err := promptLine(prompt)
	if err != nil {
		return false, err
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true, nil
	default:
		return false, nil
	}
}
