//go:build !windows && !js

package platform

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// chooseFile asks with the system chooser (osascript on macOS, zenity or kdialog on Linux) and falls back to a prompt on
// the terminal. It returns "" when cancelled.
func chooseFile(save bool, title, defaultName, desc, pattern string) (string, error) {
	var command *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		if save {
			command = exec.Command("osascript", "-e", fmt.Sprintf(`POSIX path of (choose file name with prompt %q default name %q)`, title, defaultName))
		} else {
			command = exec.Command("osascript", "-e", fmt.Sprintf(`POSIX path of (choose file with prompt %q)`, title))
		}
	default:
		if path, err := exec.LookPath("zenity"); err == nil {
			args := []string{"--file-selection", "--title=" + title, "--file-filter=" + desc + " | " + pattern}
			if save {
				args = append(args, "--save", "--confirm-overwrite", "--filename="+defaultName)
			}
			command = exec.Command(path, args...)
		} else if path, err := exec.LookPath("kdialog"); err == nil {
			if save {
				command = exec.Command(path, "--getsavefilename", defaultName, pattern+"|"+desc, "--title", title)
			} else {
				command = exec.Command(path, "--getopenfilename", ".", pattern+"|"+desc, "--title", title)
			}
		}
	}
	if command != nil {
		out, err := command.Output()
		if err == nil {
			return strings.TrimSpace(string(out)), nil
		}
		if _, isExit := err.(*exec.ExitError); isExit {
			return "", nil // cancelled
		}
	}
	fmt.Print(title + ": ")
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && line == "" {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

// OpenFileDialog asks for an existing file ("" when cancelled).
func OpenFileDialog(title, desc, pattern string) (string, error) {
	return chooseFile(false, title, "", desc, pattern)
}

// SaveFileAs asks where to save and writes data there. It returns the chosen path ("" when cancelled).
func SaveFileAs(title, defaultName, desc, pattern string, data []byte) (string, error) {
	path, err := chooseFile(true, title, defaultName, desc, pattern)
	if err != nil || path == "" {
		return "", err
	}
	return path, os.WriteFile(path, data, 0o644)
}
