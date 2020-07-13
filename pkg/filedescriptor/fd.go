package filedescriptor

import (
	"fmt"
	"io"
	"os"
)

type (
	// FD represents log file mostly
	// one that can be treated as io.WriteClose to be used within same process
	// but also to get the path to underlying file
	FD interface {
		io.WriteCloser
		File() string
	}

	fd struct {
		path string
		file *os.File
	}
)

func New(file string) (FD, error) {
	f, err := os.OpenFile(file, os.O_APPEND|os.O_WRONLY, os.ModeAppend)
	if err != nil {
		return nil, fmt.Errorf("Failed to create FD: %w", err)
	}

	return &fd{
		path: file,
		file: f,
	}, nil
}

func (f *fd) Write(p []byte) (n int, err error) {
	return f.file.Write(p)
}

func (f *fd) Close() error {
	return f.file.Close()
}

func (f *fd) File() string {
	return f.path
}