package main

import (
	"os"

	"github.com/denysvitali/odi/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
