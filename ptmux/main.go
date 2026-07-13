package main

import (
	"log"
	"os"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}

func run(args []string) error {
	if len(args) > 0 && args[0] == "kill" {
		return killCommand(args[1:])
	}
	return openCommand(args)
}
