package archive

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/iflowkit/iflowkit-cli/internal/common/filex"
	"github.com/iflowkit/iflowkit-cli/internal/models"
)

// ExportProfile zips the profile folder contents (profile.json + tenants/...) into outFile.
func ExportProfile(profileDir, profileID, outFile string) error {
	// Validate profile.json exists and is correct.
	b, err := os.ReadFile(filepath.Join(profileDir, "profile.json"))
	if err != nil {
		return err
	}
	var p models.Profile
	if err := json.Unmarshal(b, &p); err != nil {
		return fmt.Errorf("invalid profile.json: %w", err)
	}
	if err := p.ValidateRequired(); err != nil {
		return err
	}

	_ = os.Remove(outFile)
	f, err := os.OpenFile(outFile, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	defer zw.Close()

	mb, _ := json.MarshalIndent(NewManifest("profile", profileID), "", "  ")
	if err := addBytes(zw, ManifestFileName, mb); err != nil {
		return err
	}

	return filepath.WalkDir(profileDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(profileDir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasPrefix(d.Name(), ".DS_Store") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		name := filepath.ToSlash(rel)
		return addBytes(zw, name, data)
	})
}

// ExportConfig zips config.json into outFile.
func ExportConfig(configFile, outFile string) error {
	b, err := os.ReadFile(configFile)
	if err != nil {
		return err
	}
	var cfg models.Config
	if err := json.Unmarshal(b, &cfg); err != nil {
		return fmt.Errorf("invalid config.json: %w", err)
	}
	if err := cfg.ValidateRequired(); err != nil {
		return err
	}

	_ = os.Remove(outFile)
	f, err := os.OpenFile(outFile, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	defer zw.Close()

	mb, _ := json.MarshalIndent(NewManifest("config", ""), "", "  ")
	if err := addBytes(zw, ManifestFileName, mb); err != nil {
		return err
	}
	if err := addBytes(zw, "config.json", b); err != nil {
		return err
	}
	return nil
}

func PeekArchive(zipPath string) (profileID string, kind string, err error) {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", "", err
	}
	defer r.Close()

	var manifest *Manifest
	for _, f := range r.File {
		if filepath.Base(f.Name) == ManifestFileName {
			rc, err := f.Open()
			if err != nil {
				return "", "", err
			}
			b, _ := io.ReadAll(rc)
			_ = rc.Close()
			var m Manifest
			if err := json.Unmarshal(b, &m); err != nil {
				return "", "", fmt.Errorf("invalid %s: %w", ManifestFileName, err)
			}
			manifest = &m
			break
		}
	}
	if manifest != nil {
		if manifest.Kind == "" {
			return "", "", fmt.Errorf("%s missing required field: kind", ManifestFileName)
		}
		if manifest.SchemaVersion != CurrentArchiveSchemaVersion {
			return "", "", fmt.Errorf("unsupported archive schema_version %d (current: %d)", manifest.SchemaVersion, CurrentArchiveSchemaVersion)
		}
		if manifest.Kind == "profile" {
			if manifest.ProfileID != "" {
				return manifest.ProfileID, manifest.Kind, nil
			}
			// fallback to profile.json below
		} else {
			return "", manifest.Kind, nil
		}
	}

	// Fallbacks: scan for profile.json/config.json.
	var profileJSON *zip.File
	var hasConfig bool
	for _, f := range r.File {
		base := filepath.Base(f.Name)
		if base == "profile.json" {
			profileJSON = f
		}
		if base == "config.json" {
			hasConfig = true
		}
	}
	if profileJSON != nil {
		rc, err := profileJSON.Open()
		if err != nil {
			return "", "", err
		}
		b, _ := io.ReadAll(rc)
		_ = rc.Close()
		var p models.Profile
		if err := json.Unmarshal(b, &p); err != nil {
			return "", "", fmt.Errorf("invalid profile.json: %w", err)
		}
		if err := p.ValidateRequired(); err != nil {
			return "", "", err
		}
		return p.ID, "profile", nil
	}
	if hasConfig {
		return "", "config", nil
	}
	return "", "", fmt.Errorf("could not detect archive kind")

}

func findSingleFile(root, filename string) (fullPath string, dir string, err error) {
	var best string
	var bestDepth int = 1 << 30
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, werr error) error {
		if werr != nil {
			return werr
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Base(path) != filename {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		depth := strings.Count(rel, string(os.PathSeparator))
		if depth < bestDepth {
			best = path
			bestDepth = depth
		}
		return nil
	})
	if err != nil {
		return "", "", err
	}
	if best == "" {
		return "", "", fmt.Errorf("archive missing %s", filename)
	}
	return best, filepath.Dir(best), nil
}

func ImportProfile(zipPath, destDir string, overwrite bool) error {
	if err := os.MkdirAll(filepath.Dir(destDir), 0o755); err != nil {
		return err
	}
	if _, err := os.Stat(destDir); err == nil {
		if !overwrite {
			return fmt.Errorf("destination exists: %s", destDir)
		}
		if err := os.RemoveAll(destDir); err != nil {
			return err
		}
	}

	// Create temp dir in the same parent to keep rename as atomic as possible.
	parent := filepath.Dir(destDir)
	tmpDir, err := os.MkdirTemp(parent, ".iflowkit-profile-import-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	extractDir := filepath.Join(tmpDir, "extract")
	if err := os.MkdirAll(extractDir, 0o755); err != nil {
		return err
	}
	if err := extractZipSecure(zipPath, extractDir); err != nil {
		return err
	}

	// Locate profile.json (supports archives that wrap content in a top-level folder).
	profileJSON, rootDir, err := findSingleFile(extractDir, "profile.json")
	if err != nil {
		return err
	}
	b, err := os.ReadFile(profileJSON)
	if err != nil {
		return err
	}
	var p models.Profile
	if err := json.Unmarshal(b, &p); err != nil {
		return fmt.Errorf("invalid profile.json: %w", err)
	}
	if err := p.ValidateRequired(); err != nil {
		return err
	}
	if p.SchemaVersion != models.CurrentProfileSchemaVersion {
		return fmt.Errorf("unsupported profile schema_version %d (current: %d)", p.SchemaVersion, models.CurrentProfileSchemaVersion)
	}

	// Ensure tenants folder exists for consistency.
	_ = os.MkdirAll(filepath.Join(rootDir, "tenants"), 0o755)

	// Final move.
	return os.Rename(rootDir, destDir)
}

func ImportConfig(zipPath, destConfigFile string, overwrite bool) error {
	if _, err := os.Stat(destConfigFile); err == nil && !overwrite {
		return fmt.Errorf("destination exists: %s", destConfigFile)
	}

	destDir := filepath.Dir(destConfigFile)
	tmpDir, err := os.MkdirTemp(destDir, ".iflowkit-config-import-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	extractDir := filepath.Join(tmpDir, "extract")
	if err := os.MkdirAll(extractDir, 0o755); err != nil {
		return err
	}
	if err := extractZipSecure(zipPath, extractDir); err != nil {
		return err
	}

	cfgPath, _, err := findSingleFile(extractDir, "config.json")
	if err != nil {
		return err
	}
	b, err := os.ReadFile(cfgPath)
	if err != nil {
		return err
	}
	var cfg models.Config
	if err := json.Unmarshal(b, &cfg); err != nil {
		return fmt.Errorf("invalid config.json: %w", err)
	}
	if err := cfg.ValidateRequired(); err != nil {
		return err
	}
	if cfg.SchemaVersion != models.CurrentConfigSchemaVersion {
		return fmt.Errorf("unsupported config schema_version %d (current: %d)", cfg.SchemaVersion, models.CurrentConfigSchemaVersion)
	}

	_ = os.Remove(destConfigFile)
	return filex.AtomicWriteFile(destConfigFile, b, 0o644)
}

func addBytes(zw *zip.Writer, name string, b []byte) error {
	w, err := zw.Create(name)
	if err != nil {
		return err
	}
	_, err = io.Copy(w, bytes.NewReader(b))
	return err
}

// Zip Slip protection: prevents path traversal when extracting archives.
func extractZipSecure(zipPath, destDir string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()

	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}

	for _, f := range r.File {
		name := filepath.FromSlash(f.Name)
		if name == "" {
			continue
		}
		if strings.Contains(name, ":") {
			return fmt.Errorf("unsafe zip entry: %q", f.Name)
		}
		clean := filepath.Clean(name)
		if filepath.IsAbs(clean) || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) || clean == ".." {
			return fmt.Errorf("zip slip detected: %q", f.Name)
		}

		outPath := filepath.Join(destDir, clean)
		if !strings.HasPrefix(outPath, destDir+string(os.PathSeparator)) && outPath != destDir {
			return fmt.Errorf("zip slip detected: %q", f.Name)
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(outPath, 0o755); err != nil {
				return err
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			return err
		}
		data, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			return err
		}

		_ = os.Remove(outPath)
		if err := filex.AtomicWriteFile(outPath, data, 0o644); err != nil {
			return err
		}
	}

	// Manifest is recommended but optional.
	mf := filepath.Join(destDir, ManifestFileName)
	if _, err := os.Stat(mf); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
