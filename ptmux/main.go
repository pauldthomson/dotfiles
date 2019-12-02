package main

import (
	"fmt"
	"github.com/ktr0731/go-fuzzyfinder"
	"log"
	"os"
	"os/exec"
	"strings"
)

func main() {
	args := os.Args[1:]
	projectName := args[0]

	cmd := exec.Command("find", "/Users/pthomson/repos", "-maxdepth", "3", "-type", "d", "-name", projectName)
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Fatal(err)
	}

	trimmed := strings.TrimSpace(string(out))
	resultsSlice := strings.Split(trimmed, "\n")
	if len(resultsSlice) == 1 && resultsSlice[0] == "" {
		fmt.Println("Project doesn't exist")
		cmd = exec.Command("tmux", "new", "-d", "-s", projectName)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err = cmd.Run()
		if err != nil {
			log.Fatal(err)
		}
	} else {
		idx, err := fuzzyfinder.Find(resultsSlice, func(i int) string {
			return resultsSlice[i]
		})

		cmd = exec.Command("tmux", "new", "-d", "-s", projectName, "-c", resultsSlice[idx])
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err = cmd.Run()
		if err != nil {
			log.Fatal(err)
		}

		cmd = exec.Command("tmux", "send-keys", "-t", projectName, "tmux split-window -v -p20", "C-m")
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err = cmd.Run()
		if err != nil {
			log.Fatal(err)
		}

		cmd = exec.Command("tmux", "send-keys", "-t", projectName, "vim", "C-m")
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err = cmd.Run()
		if err != nil {
			log.Fatal(err)
		}
	}

	if os.Getenv("TMUX") != "" {
		cmd = exec.Command("tmux", "switch", "-t", projectName)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err = cmd.Run()
		if err != nil {
			log.Fatal(err)
		}
	} else {
		cmd = exec.Command("tmux", "a", "-t", projectName)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err = cmd.Run()
		if err != nil {
			log.Fatal(err)
		}
	}
}
