package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/ktr0731/go-fuzzyfinder"
)

func main() {
	args := os.Args[1:]

	var cmd *exec.Cmd
	var projectName string
	if len(args) == 0 {
		cmd = exec.Command("find", "/Users/pthomson/repos", "-maxdepth", "3", "-type", "d")
	} else {
		projectName = args[0]
		cmd = exec.Command("find", "/Users/pthomson/repos", "-maxdepth", "3", "-type", "d", "-name", projectName)
	}
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
		var idx int
		if len(resultsSlice) == 1 {
			idx = 0
		} else {
			idx, err = fuzzyfinder.Find(resultsSlice, func(i int) string {
				return resultsSlice[i]
			})
			if err != nil && err.Error() == "abort" {
				os.Exit(0)
			}
		}

		if projectName == "" {
			ss := strings.Split(resultsSlice[idx], "/")
			projectName = ss[len(ss)-1]
		}

		if strings.Contains(projectName, ".") {
			projectName = strings.ReplaceAll(projectName, ".", "-")
		}

		cmd = exec.Command("tmux", "new", "-d", "-s", projectName, "-c", resultsSlice[idx])
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err = cmd.Run()
		if err != nil {
			log.Fatal(err)
		}

		// cmd = exec.Command("tmux", "send-keys", "-t", projectName, "tmux split-window -v -p20", "C-m")
		// cmd.Stdin = os.Stdin
		// cmd.Stdout = os.Stdout
		// cmd.Stderr = os.Stderr
		// err = cmd.Run()
		// if err != nil {
		// 	log.Fatal(err)
		// }

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
