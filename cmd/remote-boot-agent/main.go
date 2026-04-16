package main

import (
	"os"

	"github.com/jjack/remote-boot-agent/internal/bootloader"
	"github.com/jjack/remote-boot-agent/internal/bootloader/grub"
)

func main() {
	blReg := bootloader.NewRegistry(grub.New())

	rootCmd := newRootCmd(blReg)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
