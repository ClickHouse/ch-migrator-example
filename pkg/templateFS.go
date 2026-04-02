package migrations

import (
	"bytes"
	"io"
	"io/fs"
	"sort"
)

// templatedFS wraps around an embed.FS but gives us a hook to make string
// replacements to the text on the fly.
type templatedFS struct {
	fs           fs.FS
	replacements map[string]string
}

// templatedFile wraps a file and applies template replacements during Read.
// On the first Read call it reads the entire underlying file, applies all
// replacements, and then serves subsequent reads from the expanded buffer.
type templatedFile struct {
	file         fs.File
	replacements map[string]string
	buf          []byte // lazily initialized with fully-replaced content
	offset       int
}

func (t templatedFS) ReadDir(name string) ([]fs.DirEntry, error) {
	return fs.ReadDir(t.fs, name)
}

func (t templatedFS) Open(name string) (fs.File, error) {
	if t.fs == nil {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}
	fd, err := t.fs.Open(name)
	if err != nil {
		return nil, err
	}
	return &templatedFile{file: fd, replacements: t.replacements}, nil
}

func (t templatedFS) ReadFile(name string) ([]byte, error) {
	f, err := t.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
}

func (t *templatedFile) Stat() (fs.FileInfo, error) {
	return t.file.Stat()
}

func (t *templatedFile) Read(bs []byte) (int, error) {
	if t.buf == nil {
		raw, err := io.ReadAll(t.file)
		if err != nil {
			return 0, err
		}
		// Sort replacement keys for deterministic ordering.
		keys := make([]string, 0, len(t.replacements))
		for k := range t.replacements {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			raw = bytes.ReplaceAll(raw, []byte(k), []byte(t.replacements[k]))
		}
		t.buf = raw
		t.offset = 0
	}

	if t.offset >= len(t.buf) {
		return 0, io.EOF
	}

	n := copy(bs, t.buf[t.offset:])
	t.offset += n
	if t.offset >= len(t.buf) {
		return n, io.EOF
	}
	return n, nil
}

func (t *templatedFile) Close() error {
	return t.file.Close()
}
