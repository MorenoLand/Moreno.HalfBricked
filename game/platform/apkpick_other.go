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

// PickAPK asks for the APK with the system's file chooser (osascript on macOS, zenity or kdialog on Linux) and falls
// back to a prompt on the terminal. It returns "" when the player cancels.
func PickAPK() (string, error) {
	var command *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		command = exec.Command("osascript", "-e", `POSIX path of (choose file with prompt "Select the Age of Zombies APK")`)
	default:
		if path, err := exec.LookPath("zenity"); err == nil {
			command = exec.Command(path, "--file-selection", "--title=Select the Age of Zombies APK", "--file-filter=Android package | *.apk")
		} else if path, err := exec.LookPath("kdialog"); err == nil {
			command = exec.Command(path, "--getopenfilename", ".", "*.apk|Android package", "--title", "Select the Age of Zombies APK")
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
	fmt.Print("Path to the Age of Zombies APK: ")
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && line == "" {
		return "", err
	}
	return strings.TrimSpace(line), nil
}
