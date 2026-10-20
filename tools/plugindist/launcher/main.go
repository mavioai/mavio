// Command launcher is the executable plugin.exe of a Python plugin on
// Windows, which runs no shell scripts: it runs the console script the
// plugin's project names after itself on the folder's Python, isolated
// from the environment's, with its own arguments, standard streams and
// exit code. plugindist builds it for each plugin, setting module and
// function.
//
// The Python process reads the launcher's standard input, the plugin's
// lifeline, so it ends when the host closes it, even after the launcher
// was killed.
package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// module and function are the console script's, set by the linker.
var module, function string

func main() {
	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, "launcher:", err)
		os.Exit(1)
	}
	code := fmt.Sprintf("import sys; from %s import %s; sys.exit(%s())", module, function, function)
	args := append([]string{"-I", "-c", code}, os.Args[1:]...)
	cmd := exec.Command(filepath.Join(filepath.Dir(exe), "python", "python.exe"), args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		if exit, ok := errors.AsType[*exec.ExitError](err); ok {
			os.Exit(exit.ExitCode())
		}
		fmt.Fprintln(os.Stderr, "launcher:", err)
		os.Exit(1)
	}
}
