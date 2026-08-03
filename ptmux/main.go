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
	if len(args) > 0 {
		switch args[0] {
		case "kill":
			return killCommand(args[1:])
		case "_cleanup-cloned-session":
			if len(args) != 3 {
				return cleanupClonedSessionUsageError()
			}
			return cleanupClonedSessionCommand(args[1], args[2])
		}
	}
	return openCommand(args)
}
