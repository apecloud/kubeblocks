package procfs

import (
	"os"
	"path"
)

type ProcFS interface {
	Set(key, value string) error

	Get(key string) (string, error)
}

type procFs struct {
}

func (p *procFs) path(key string) string {
	return path.Join("/proc/sys", key)
}

func (p *procFs) Set(key, value string) error {
	return os.WriteFile(p.path(key), []byte(value), 0644)
}

func (p *procFs) Get(key string) (string, error) {
	data, err := os.ReadFile(p.path(key))
	return string(data), err
}

func NewProcFS() ProcFS {
	return &procFs{}
}
