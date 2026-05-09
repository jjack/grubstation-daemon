package main

import (
	"log/slog"
	"os"

	"github.com/jjack/grubstation-cli/internal/cli"
)

func main() {
	app := cli.NewCLI()
	if err := app.Execute(); err != nil {
		slog.Error("Error executing command", "error", err)
		os.Exit(1)
	}
}
