package files

import (
	"github.com/SkyPanel/SkyPanel/v3/internal/utils"
	"io/fs"
	"os"
)

type MergedFS struct {
	fileSystems []fs.FS
}

func NewMergedFS(systems ...fs.FS) *MergedFS {
	return &MergedFS{
		fileSystems: systems,
	}
}

type Filesystem interface {
	fs.FS
	fs.ReadDirFS
	fs.ReadFileFS
}

func (m *MergedFS) Open(name string) (fs.File, error) {
	var f fs.File
	var e error
	for _, fss := range m.fileSystems {
		f, e = fss.Open(name)
		if e == nil {
			return f, nil
		}
		if !os.IsNotExist(e) {
			return nil, e
		}
	}
	return f, e
}

func (m *MergedFS) ReadDir(name string) ([]fs.DirEntry, error) {
	results := make([]fs.DirEntry, 0)

	for _, fss := range m.fileSystems {
		if dirFs, ok := fss.(fs.ReadDirFS); ok {
			secondary, err := dirFs.ReadDir(name)
			if err != nil && !os.IsNotExist(err) {
				return nil, err
			}

			results = appendMissingEntries(results, secondary)
		}
	}

	return results, nil
}

// appendMissingEntries adds the entries whose name is not in results yet, so earlier file
// systems win.
func appendMissingEntries(results, entries []fs.DirEntry) []fs.DirEntry {
	for _, v := range entries {
		if !containsEntry(results, v.Name()) {
			results = append(results, v)
		}
	}
	return results
}

func containsEntry(entries []fs.DirEntry, name string) bool {
	for _, z := range entries {
		if z.Name() == name {
			return true
		}
	}
	return false
}

func (m *MergedFS) ReadFile(name string) (data []byte, err error) {
	for _, fss := range m.fileSystems {
		if readFs, ok := fss.(fs.ReadFileFS); ok {
			data, err = readFs.ReadFile(name)
			if err == nil || !os.IsNotExist(err) {
				return
			}
		} else {
			// primary FS does not expose the read file endpoint. We must read it directly
			data, err = read(fss, name)
			if err == nil || !os.IsNotExist(err) {
				return
			}
		}
	}

	return nil, os.ErrNotExist
}

func read(fss fs.FS, name string) ([]byte, error) {
	f, err := fss.Open(name)
	defer utils.Close(f)

	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	// to get the buffer... we have to make it the right size
	var fi fs.FileInfo
	fi, err = f.Stat()
	if err != nil {
		return nil, err
	}
	data := make([]byte, fi.Size())
	_, err = f.Read(data)
	return data, err
}
