//go:build windows

package logging

import (
	"io"
	"os"
	"path"
	"sync"
)

type Rotator struct {
	sync.RWMutex
	io.WriteCloser
	backer io.WriteCloser
}

func (r *Rotator) Write(p []byte) (n int, err error) {
	return len(p), nil
}

func (r *Rotator) Close() error {
	if r.backer != nil {
		return r.backer.Close()
	}
	return nil
}

func (r *Rotator) Rotate(newBackend io.WriteCloser) {
	r.Lock()
	defer r.Unlock()
	oldBacker := r.backer
	r.backer = newBackend
	_ = oldBacker.Close()
}

func (r *Rotator) StartRotation(dir string) {
	_ = os.MkdirAll(dir, 0755)
	newFile, err := os.OpenFile(path.Join(dir, "skypanel.log"), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err == nil {
		r.Rotate(newFile)
	}
}
