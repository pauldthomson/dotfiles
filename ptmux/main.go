package main

import (
	"bytes"
	"fmt"
	fuzzyfinder "github.com/ktr0731/go-fuzzyfinder"
	"os"
	"os/exec"
	"strings"
)

func main() {
	args := os.Args[1:]
	project := args[0]

	cmd := exec.Command("/usr/bin/find", "/Users/pthomson/repos",
		"-maxdepth", "3", "-type", "d",
		"-name", project)

	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		fmt.Println(fmt.Sprint(err) + ": " + stderr.String())
		return
	}

	results := strings.Split(out.String(), "\n")
	idx, err := fuzzyfinder.Find(results, func(i int) string {
		return results[i]
	})

	cmd = exec.Command("/usr/local/bin/tmux", "new", "-d", "-s", project, "-c", results[idx])
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	err = cmd.Run()
	if err != nil {
		fmt.Println(fmt.Sprint(err) + ": " + stderr.String())
		return
	}

	cmd = exec.Command("/usr/local/bin/tmux", "send-keys", "-t", project, "tmux split-window -v -p20", "C-m")
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	err = cmd.Run()
	if err != nil {
		fmt.Println(fmt.Sprint(err) + ": " + stderr.String())
		return
	}

	cmd = exec.Command("/usr/local/bin/tmux", "send-keys", "-t", project, "vim", "C-m")
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	err = cmd.Run()
	if err != nil {
		fmt.Println(fmt.Sprint(err) + ": " + stderr.String())
		return
	}

	if os.Getenv("TMUX") != "" {
		cmd = exec.Command("/usr/local/bin/tmux", "switch", "-t", project)
		cmd.Stdout = &out
		cmd.Stderr = &stderr
		err = cmd.Run()
		if err != nil {
			fmt.Println(fmt.Sprint(err) + ": " + stderr.String())
			return
		}
	} else {
		cmd = exec.Command("/usr/local/bin/tmux", "a", "-t", project)
		cmd.Stdout = &out
		cmd.Stderr = &stderr
		err = cmd.Run()
		if err != nil {
			fmt.Println(fmt.Sprint(err) + ": " + stderr.String())
			return
		}
	}
}
