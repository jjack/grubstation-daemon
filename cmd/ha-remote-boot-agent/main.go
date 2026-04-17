package main

import (
	"fmt"
	"os"
)

func main() {
	app := NewCLI()
	if err := app.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
