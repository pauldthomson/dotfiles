package main

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

type commandRunner interface {
	Output(name string, args ...string) (string, error)
	Run(name string, args ...string) error
	Silent(name string, args ...string) error
}

type systemCommandRunner struct{}

var commands commandRunner = systemCommandRunner{}

func (systemCommandRunner) Output(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), commandError(name, args, out, err)
	}
	return string(out), nil
}

func (systemCommandRunner) Run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return commandError(name, args, nil, err)
	}
	return nil
}

func (systemCommandRunner) Silent(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	if err := cmd.Run(); err != nil {
		return commandError(name, args, nil, err)
	}
	return nil
}

func commandError(name string, args []string, output []byte, err error) error {
	parts := make([]string, 0, len(args)+1)
	parts = append(parts, strconv.Quote(name))
	for _, arg := range args {
		parts = append(parts, strconv.Quote(arg))
	}

	message := fmt.Sprintf("command %s failed", strings.Join(parts, " "))
	if detail := strings.TrimSpace(string(output)); detail != "" {
		message += ": " + detail
	}
	return fmt.Errorf("%s: %w", message, err)
}

func outputCmd(name string, args ...string) (string, error) {
	return commands.Output(name, args...)
}

func runCmd(name string, args ...string) error {
	return commands.Run(name, args...)
}

func runSilent(name string, args ...string) error {
	return commands.Silent(name, args...)
}
