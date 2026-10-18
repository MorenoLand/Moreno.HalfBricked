package game

import (
	"log"
	"os"
	"path/filepath"

	"github.com/MorenoLand/Moreno.HalfBricked/game/platform"
)

// setupLogging keeps the log in a file when the program has no console (the Windows GUI build), so a problem can be
// looked at afterwards.
func setupLogging() {
	if platform.HasConsole() {
		return
	}
	path := platform.LogFilePath()
	if path == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	if file, err := os.Create(path); err == nil {
		log.SetOutput(file)
	}
}

// fatal ends the program with the reason, in a message box when there is no console to read it in.
func fatal(err error) {
	log.Print(err)
	if !platform.HasConsole() {
		platform.ShowError("HalfBricked", err.Error()+"\n\nLog: "+platform.LogFilePath())
	}
	os.Exit(1)
}
