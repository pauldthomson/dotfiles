package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

func promptLine(prompt string) (string, error) {
	fmt.Print(prompt)
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
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

func showWarning(message string) {
	fmt.Fprintln(os.Stderr, message)
	if os.Getenv("TMUX") != "" {
		_, _ = promptLine("\nPress enter to close this warning...")
	}
}
