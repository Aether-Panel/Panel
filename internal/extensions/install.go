package extensions

import (
	"archive/zip"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"gorm.io/gorm"
)

// Package limits: an extension is a few pages, not a place to store data
const (
	MaxPackageSize = 20 << 20
	maxFiles       = 500
	maxUnpacked    = 50 << 20
)

// ErrNotFound is an extension that is not installed
var ErrNotFound = errors.New("extension not installed")

// Folder is where extension id keeps its files
func Folder(id string) string {
	return filepath.Join(config.ExtensionsFolder.Value(), id)
}

// Install unpacks a package and records it. Installing an id that is already there updates
// it and keeps its switch, secret and a webhook the administrator changed. A new extension
// starts switched off, so nothing runs or receives events until an administrator turns it on.
func Install(db *gorm.DB, archive io.ReaderAt, size int64) (*models.Extension, error) {
	if size > MaxPackageSize {
		return nil, fmt.Errorf("the package is larger than %d MB", MaxPackageSize>>20)
	}
	zr, err := zip.NewReader(archive, size)
	if err != nil {
		return nil, errors.New("the package is not a zip file")
	}

	files, prefix, err := packageFiles(zr)
	if err != nil {
		return nil, err
	}
	manifestFile, ok := files[ManifestFile]
	if !ok {
		return nil, fmt.Errorf("%s is missing from the package", ManifestFile)
	}
	data, err := readLimited(manifestFile, 64<<10)
	if err != nil {
		return nil, err
	}
	manifest, err := ParseManifest(data, func(name string) bool { _, ok := files[name]; return ok })
	if err != nil {
		return nil, err
	}

	var existing models.Extension
	err = db.First(&existing, "id = ?", manifest.ID).Error
	updating := err == nil
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	if err := unpack(files, prefix, manifest.ID); err != nil {
		return nil, err
	}

	ext := &models.Extension{
		ID:          manifest.ID,
		Name:        manifest.Name,
		Version:     manifest.Version,
		Author:      manifest.Author,
		Description: manifest.Description,
		Manifest:    string(data),
		WebhookURL:  manifest.Webhook,
	}
	if updating {
		ext.Enabled = existing.Enabled
		ext.Secret = existing.Secret
		ext.CreatedAt = existing.CreatedAt
		if previous, err := existingManifest(&existing); err == nil && existing.WebhookURL != previous.Webhook {
			ext.WebhookURL = existing.WebhookURL
		}
	} else if ext.Secret, err = newSecret(); err != nil {
		return nil, err
	}

	if err := db.Save(ext).Error; err != nil {
		return nil, err
	}
	Reload(db)
	return ext, nil
}

// Uninstall removes the extension's files and record
func Uninstall(db *gorm.DB, id string) error {
	res := db.Delete(&models.Extension{}, "id = ?", id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	Reload(db)
	if !idPattern.MatchString(id) {
		return nil
	}
	return os.RemoveAll(Folder(id))
}

// ManifestOf reads the manifest an installed extension was recorded with
func ManifestOf(ext *models.Extension) (*Manifest, error) {
	return existingManifest(ext)
}

func existingManifest(ext *models.Extension) (*Manifest, error) {
	// it was checked when installed; the files it names are on disk
	return ParseManifest([]byte(ext.Manifest), func(string) bool { return true })
}

// packageFiles maps every file of the package to its path inside it. A zip made by
// compressing the extension's folder has everything under that folder: the prefix is
// dropped so extension.json is at the root either way.
func packageFiles(zr *zip.Reader) (map[string]*zip.File, string, error) {
	prefix := ""
	if _, ok := find(zr, ManifestFile); !ok {
		for _, f := range zr.File {
			if dir, name := path.Split(f.Name); name == ManifestFile && strings.Count(dir, "/") == 1 {
				prefix = dir
				break
			}
		}
	}

	files := map[string]*zip.File{}
	var total uint64
	for _, f := range zr.File {
		if f.FileInfo().IsDir() || strings.HasPrefix(path.Base(f.Name), "__MACOSX") || strings.HasPrefix(f.Name, "__MACOSX/") {
			continue
		}
		if f.Mode()&os.ModeSymlink != 0 {
			return nil, "", fmt.Errorf("%s: links are not allowed in a package", f.Name)
		}
		if !strings.HasPrefix(f.Name, prefix) {
			return nil, "", fmt.Errorf("%s is outside the extension's folder", f.Name)
		}
		name := strings.TrimPrefix(f.Name, prefix)
		if !isPackagePath(name) {
			return nil, "", fmt.Errorf("%s is not a valid file name", f.Name)
		}
		total += f.UncompressedSize64
		if len(files) >= maxFiles || total > maxUnpacked {
			return nil, "", fmt.Errorf("a package holds at most %d files and %d MB", maxFiles, maxUnpacked>>20)
		}
		files[name] = f
	}
	return files, prefix, nil
}

func find(zr *zip.Reader, name string) (*zip.File, bool) {
	for _, f := range zr.File {
		if f.Name == name {
			return f, true
		}
	}
	return nil, false
}

func readLimited(f *zip.File, limit int64) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	data, err := io.ReadAll(io.LimitReader(rc, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("%s is too large", f.Name)
	}
	return data, nil
}

// unpack writes the files into a fresh folder and then swaps it in, so a failed update
// leaves the installed version untouched
func unpack(files map[string]*zip.File, prefix, id string) error {
	// the manifest check already did this; it is repeated where the id becomes a folder name
	// so nothing can reach here with one that climbs out of the extensions folder
	if !idPattern.MatchString(id) || strings.Contains(id, "..") {
		return fmt.Errorf("%q is not a valid extension id", id)
	}
	root := config.ExtensionsFolder.Value()
	if err := os.MkdirAll(root, 0755); err != nil {
		return err
	}
	staging, err := os.MkdirTemp(root, ".install-"+id+"-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(staging)

	var written int64
	for name, f := range files {
		target := filepath.Join(staging, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		n, err := writeFile(f, target, maxUnpacked-written)
		if err != nil {
			return fmt.Errorf("%s: %w", strings.TrimPrefix(f.Name, prefix), err)
		}
		written += n
	}

	final := Folder(id)
	old := final + ".old"
	_ = os.RemoveAll(old)
	if err := os.Rename(final, old); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.Rename(staging, final); err != nil {
		_ = os.Rename(old, final)
		return err
	}
	return os.RemoveAll(old)
}

// writeFile copies one entry, refusing to write more than budget bytes: the sizes a zip
// declares are not to be trusted
func writeFile(f *zip.File, target string, budget int64) (int64, error) {
	rc, err := f.Open()
	if err != nil {
		return 0, err
	}
	defer rc.Close()
	out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0644)
	if err != nil {
		return 0, err
	}
	n, err := io.Copy(out, io.LimitReader(rc, budget+1))
	if closeErr := out.Close(); err == nil {
		err = closeErr
	}
	if err == nil && n > budget {
		err = fmt.Errorf("the package unpacks to more than %d MB", maxUnpacked>>20)
	}
	return n, err
}

func newSecret() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
}
