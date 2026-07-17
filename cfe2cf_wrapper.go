package main

import (
	"os"
	"os/exec"
	"path/filepath"
)

func main() {
	exePath, _ := os.Executable()
	exeDir := filepath.Dir(exePath)
	scriptPath := filepath.Join(exeDir, "..", "src", "cfe2cf.os")

	cmd := exec.Command("oscript", append([]string{scriptPath}, os.Args[1:]...)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		os.Exit(1)
	}
}
