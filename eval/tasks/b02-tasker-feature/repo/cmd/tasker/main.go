package main

import (
	"os"
	"time"

	"example.com/tasker/internal/cli"
)

func main() {
	env := cli.Env{
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		File:   os.Getenv("TASKER_FILE"),
		Now:    time.Now,
	}
	os.Exit(cli.Run(os.Args[1:], env))
}
