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
		cmd = exec.Command("find", os.ExpandEnv("${HOME}")+"/repos", "-maxdepth", "3", "-mindepth", "3", "-type", "d")
	} else {
		projectName = args[0]
		cmd = exec.Command("find", os.ExpandEnv("${HOME}")+"/repos", "-maxdepth", "3", "-mindepth", "3", "-type", "d", "-name", projectName)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Fatal(string(out))
	}

	trimmed := strings.TrimSpace(string(out))
	resultsSlice := strings.Split(trimmed, "\n")
	if len(resultsSlice) == 1 && resultsSlice[0] == "" {
		fmt.Println("Project doesn't exist")
		err := runCmd("tmux", "new", "-d", "-s", projectName)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		for i, p := range resultsSlice {
			sp := strings.Split(p, string(os.PathSeparator))
			resultsSlice[i] = strings.Join(sp[len(sp)-2:], string(os.PathSeparator))
		}

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

		if err := runCmd("tmux", "has-session", fmt.Sprintf("-t=%s", projectName)); err != nil {
			//TODO: don't assume github.com
			startDir := strings.Join([]string{os.ExpandEnv("${HOME}"), "repos", "github.com", resultsSlice[idx]}, string(os.PathSeparator))
			err := runCmd("tmux", "new", "-d", "-s", projectName, "-c", startDir)
			if err != nil {
				log.Fatal(err)
			}

			err = runCmd("tmux", "new-window", "-n", "opencode", "-t", projectName, "-c", startDir)
			if err != nil {
				log.Fatal(err)
			}

			err = runCmd("tmux", "send-keys", "-t", fmt.Sprintf("%s:3", projectName), "opencode", "C-m")
			if err != nil {
				log.Fatal(err)
			}

			err = runCmd("tmux", "new-window", "-n", "scratch", "-t", projectName, "-c", startDir)
			if err != nil {
				log.Fatal(err)
			}

			err = runCmd("tmux", "send-keys", "-t", fmt.Sprintf("%s:1", projectName), "nvim", "C-m")
			if err != nil {
				log.Fatal(err)
			}
		}
	}

	if os.Getenv("TMUX") != "" {
		runCmd("tmux", "switch", "-t", fmt.Sprintf("%s:1", projectName))
	} else {
		runCmd("tmux", "a", "-t", fmt.Sprintf("%s:1", projectName))
	}
}

func runCmd(cmd string, args ...string) error {
	c := exec.Command(cmd, args...)
	c.Stdin = os.Stdin
	return c.Run()
}
