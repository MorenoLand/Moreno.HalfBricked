package content

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// importerVersion changes whenever the importer starts caching new asset kinds,
// so an up-to-date APK hash alone does not keep an incomplete cache.
const importerVersion = "2-uiscreens"

// ImportStarted and ImportFinished, when set, are called around an import that is really going to happen (not when the
// cache is already up to date), so the desktop program can show that it is busy.
var ImportStarted, ImportFinished func()

func prepareAPK(apkPath string) (string, error) {
	return PrepareAPK(apkPath, filepath.Join("bin", "data-cache"))
}
func PrepareAPK(apkPath, output string) (string, error) {
	marker := filepath.Join(output, ".apk-source.sha256")
	hash, err := HashFile(apkPath)
	if err != nil {
		return "", fmt.Errorf("hash APK %q: %w", apkPath, err)
	}
	if _, err := os.Stat(filepath.Join(output, "pack.json")); err == nil {
		if data, readErr := os.ReadFile(marker); readErr == nil && strings.TrimSpace(string(data)) == hash+" "+importerVersion && hasScriptLevelCache(output) {
			return output, nil
		}
	}
	if ImportStarted != nil {
		ImportStarted()
	}
	if ImportFinished != nil {
		defer ImportFinished()
	}
	stage := filepath.Join(filepath.Dir(output), ".apk-assets")
	if err := os.MkdirAll(filepath.Dir(stage), 0755); err != nil {
		return "", err
	}
	if err := os.RemoveAll(stage); err != nil {
		return "", err
	}
	defer os.RemoveAll(stage)
	if err := extractAPKAssets(apkPath, stage); err != nil {
		return "", err
	}
	if err := Import(stage, output); err != nil {
		return "", fmt.Errorf("import APK assets: %w", err)
	}
	if err := os.WriteFile(marker, []byte(hash+" "+importerVersion+"\n"), 0644); err != nil {
		return "", err
	}
	return output, nil
}

func hasScriptLevelCache(root string) bool {
	_, err := os.Stat(filepath.Join(root, "levels", "world0_lab.json"))
	return err == nil
}

func extractAPKAssets(apkPath, stage string) error {
	archive, err := zip.OpenReader(apkPath)
	if err != nil {
		return fmt.Errorf("open APK %q: %w", apkPath, err)
	}
	defer archive.Close()
	found := false
	stageRoot, err := filepath.Abs(stage)
	if err != nil {
		return err
	}
	for index, entry := range archive.File {
		progress("Unpacking the APK", index, len(archive.File))
		name := strings.TrimPrefix(strings.ReplaceAll(entry.Name, "\\", "/"), "./")
		clean := path.Clean(name)
		if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || strings.HasPrefix(clean, "/") {
			return fmt.Errorf("unsafe APK entry %q", entry.Name)
		}
		parts := strings.SplitN(clean, "/", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "assets") {
			continue
		}
		relative := path.Clean(parts[1])
		if relative == "." || relative == ".." || strings.HasPrefix(relative, "../") || strings.HasPrefix(relative, "/") || strings.Contains(strings.Split(relative, "/")[0], ":") {
			return fmt.Errorf("unsafe APK asset entry %q", entry.Name)
		}
		if entry.Mode()&os.ModeSymlink != 0 || entry.FileInfo().IsDir() {
			continue
		}
		destination := filepath.Join(stage, "assets", filepath.FromSlash(relative))
		destinationAbs, err := filepath.Abs(destination)
		if err != nil {
			return err
		}
		prefix := stageRoot + string(os.PathSeparator)
		if !strings.EqualFold(destinationAbs, stageRoot) && !strings.HasPrefix(strings.ToLower(destinationAbs), strings.ToLower(prefix)) {
			return fmt.Errorf("unsafe APK destination %q", entry.Name)
		}
		if err := os.MkdirAll(filepath.Dir(destination), 0755); err != nil {
			return err
		}
		input, err := entry.Open()
		if err != nil {
			return fmt.Errorf("read APK entry %q: %w", entry.Name, err)
		}
		output, err := os.OpenFile(destination, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
		if err != nil {
			input.Close()
			return err
		}
		_, copyErr := io.Copy(output, input)
		inputCloseErr := input.Close()
		outputCloseErr := output.Close()
		if copyErr != nil {
			return fmt.Errorf("extract APK entry %q: %w", entry.Name, copyErr)
		}
		if inputCloseErr != nil {
			return inputCloseErr
		}
		if outputCloseErr != nil {
			return outputCloseErr
		}
		found = true
	}
	if !found {
		return fmt.Errorf("APK contains no runtime assets directory")
	}
	return nil
}

// ValidateAPK reports whether path is a package holding the game's assets (a zip with an assets/ tree and the
// level catalogue), so a wrong file is refused before anything is imported.
func ValidateAPK(apkPath string) error {
	archive, err := zip.OpenReader(apkPath)
	if err != nil {
		return fmt.Errorf("%q is not an APK: %w", apkPath, err)
	}
	defer archive.Close()
	for _, entry := range archive.File {
		name := strings.ToLower(strings.ReplaceAll(entry.Name, "\\", "/"))
		if strings.HasPrefix(name, "assets/world0/levels/") {
			return nil
		}
	}
	return fmt.Errorf("%q does not hold the Age of Zombies assets", apkPath)
}
