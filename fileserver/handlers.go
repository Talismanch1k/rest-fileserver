// Package main implements a file storage server, relies on unix filesystem semantics (on Windows can be undefined behaviour)
package main

import (
	"cmp"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"hash/maphash"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"sync"
)

const (
	locksMapSize     = 1024
	internalErrorMsg = "internal error"
)

var errInvalidFilename = errors.New("invalid filename")

type fileStorage struct {
	uploadDir   *os.Root // temp folder in uploadDir/tmp
	maxFileSize int64
	locks       [locksMapSize]sync.RWMutex // lock striping pattern
	seed        maphash.Seed
}

func (fs *fileStorage) fileMutex(filename string) *sync.RWMutex {
	idx := maphash.String(fs.seed, filename) % locksMapSize
	return &fs.locks[idx]
}

// no mutex file listing (like nginx and others)
func (fs *fileStorage) listFilesHandler(w http.ResponseWriter, r *http.Request) {
	dir, err := fs.uploadDir.Open(".")
	if err != nil {
		slog.Error("failed to open upload dir", "err", err)
		http.Error(w, internalErrorMsg, http.StatusInternalServerError)
		return
	}
	defer func() {
		if cerr := dir.Close(); cerr != nil {
			slog.Error("failed to close upload dir", "err", cerr)
		}
	}()

	entries, err := dir.ReadDir(-1)
	if err != nil {
		slog.Error("failed to read upload dir", "err", err)
		http.Error(w, internalErrorMsg, http.StatusInternalServerError)
		return
	}

	slices.SortFunc(entries, func(a, b os.DirEntry) int {
		return cmp.Compare(a.Name(), b.Name())
	})

	for _, entry := range entries {
		if !entry.IsDir() {
			_, err = fmt.Fprintln(w, entry.Name())
			if err != nil {
				slog.Error("failed to write response", "err", err)
				return
			}
		}
	}
}

func (fs *fileStorage) getFileHandler(w http.ResponseWriter, r *http.Request) {
	filename, err := fs.extractFilename(r.PathValue("filename"))
	if err != nil {
		http.Error(w, "invalid filename", http.StatusBadRequest)
		return
	}

	mu := fs.fileMutex(filename)

	// mutex only for inode descriptor
	mu.RLock()
	file, err := fs.uploadDir.Open(filename)
	mu.RUnlock()

	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			http.Error(w, "file not found", http.StatusNotFound)
			return
		}
		slog.Error("failed to open file", "path", fs.uploadDir.Name(), "err", err)
		http.Error(w, internalErrorMsg, http.StatusInternalServerError)
		return
	}

	defer func() {
		if cerr := file.Close(); cerr != nil {
			slog.Error("failed to close file", "path", fs.uploadDir.Name(), "err", cerr)
		}
	}()

	if _, err := io.Copy(w, file); err != nil {
		slog.Error("failed to write response", "path", fs.uploadDir.Name(), "err", err)
		return
	}
}

func (fs *fileStorage) uploadFileHandler(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, fs.maxFileSize)

	file, header, err := r.FormFile("file")
	if err != nil {
		var maxBytesErr *http.MaxBytesError

		if errors.As(err, &maxBytesErr) {
			http.Error(w, "file too large", http.StatusRequestEntityTooLarge)
			return
		}

		http.Error(w, "invalid file", http.StatusBadRequest)
		return
	}
	defer func() {
		if cerr := file.Close(); cerr != nil {
			slog.Error("failed close file", "err", cerr)
		}
	}()

	filename, err := fs.extractFilename(header.Filename)
	if err != nil {
		http.Error(w, "invalid filename", http.StatusBadRequest)
		return
	}

	tempFilename, clean, err := fs.writeTempFile(file, filename)
	if err != nil {
		slog.Error("failed to write temp file", "err", err)
		http.Error(w, internalErrorMsg, http.StatusInternalServerError)
		return
	}
	defer clean()

	mu := fs.fileMutex(filename)
	mu.Lock()
	defer mu.Unlock()

	if _, err := fs.uploadDir.Stat(filename); err == nil {
		http.Error(w, "file already exists", http.StatusConflict)
		return
	} else if !errors.Is(err, os.ErrNotExist) {
		slog.Error("failed to stat file", "err", err)
		http.Error(w, internalErrorMsg, http.StatusInternalServerError)
		return
	}

	if err := fs.uploadDir.Rename(tempFilename, filename); err != nil {
		slog.Error("failed move temp file to upload dir", "err", err)
		http.Error(w, internalErrorMsg, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	_, err = fmt.Fprintln(w, filename)
	if err != nil {
		slog.Error("failed to write response", "err", err)
		return
	}
}

func (fs *fileStorage) deleteFileHandler(w http.ResponseWriter, r *http.Request) {
	filename, err := fs.extractFilename(r.PathValue("filename"))
	if err != nil {
		http.Error(w, "invalid filename", http.StatusBadRequest)
		return
	}

	mu := fs.fileMutex(filename)
	mu.Lock()
	defer mu.Unlock()

	err = fs.uploadDir.Remove(filename)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		http.Error(w, internalErrorMsg, http.StatusInternalServerError)
		return
	}
}

func (fs *fileStorage) updateFileHandler(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, fs.maxFileSize)

	filename, err := fs.extractFilename(r.PathValue("filename"))
	if err != nil {
		http.Error(w, "invalid filename", http.StatusBadRequest)
		return
	}

	// optimistic check
	if _, err := fs.uploadDir.Stat(filename); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			http.Error(w, "file not found", http.StatusNotFound)
			return
		}
		slog.Error("failed to stat file", "err", err)
		http.Error(w, internalErrorMsg, http.StatusInternalServerError)
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		var maxBytesErr *http.MaxBytesError

		if errors.As(err, &maxBytesErr) {
			http.Error(w, "file too large", http.StatusRequestEntityTooLarge)
			return
		}

		http.Error(w, "invalid file", http.StatusBadRequest)
		return
	}
	defer func() {
		if cerr := file.Close(); cerr != nil {
			slog.Error("failed close file", "err", cerr)
		}
	}()

	tempFilename, clean, err := fs.writeTempFile(file, filename)
	if err != nil {
		slog.Error("failed to write temp file", "err", err)
		http.Error(w, internalErrorMsg, http.StatusInternalServerError)
		return
	}
	defer clean()

	mu := fs.fileMutex(filename)
	mu.Lock()
	defer mu.Unlock()

	// recheck if file still exists
	if _, err := fs.uploadDir.Stat(filename); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			http.Error(w, "file not found", http.StatusNotFound)
			return
		}
		slog.Error("failed to stat file", "err", err)
		http.Error(w, internalErrorMsg, http.StatusInternalServerError)
		return
	}

	if err := fs.uploadDir.Rename(tempFilename, filename); err != nil {
		slog.Error("failed to replace old file with new one", "err", err)
		http.Error(w, internalErrorMsg, http.StatusInternalServerError)
		return
	}
}

func (fs *fileStorage) extractFilename(filename string) (string, error) {
	filename = filepath.Base(filename)
	if filename == "." || filename == ".." || filename == "" || filename == "tmp" {
		return "", errInvalidFilename
	}

	return filename, nil
}

func (fs *fileStorage) tempFilename(filename string) (string, error) {
	b := make([]byte, 8)

	if _, err := rand.Read(b); err != nil {
		return "", err
	}

	return filepath.Join("tmp", filename+"."+hex.EncodeToString(b)+".tmp"), nil
}

func (fs *fileStorage) writeTempFile(src io.Reader, filename string) (tempPath string, clean func(), err error) {
	tempFilename, err := fs.tempFilename(filename)
	if err != nil {
		return "", nil, fmt.Errorf("generating temp filename: %w", err)
	}

	dst, err := fs.uploadDir.OpenFile(tempFilename, os.O_EXCL|os.O_WRONLY|os.O_CREATE, 0o644)
	if err != nil {
		return "", nil, fmt.Errorf("creating temp file: %w", err)
	}

	clean = func() {
		if err := dst.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
			slog.Error("failed to close temp file", "path", tempFilename, "err", err)
		}
		if err := fs.uploadDir.Remove(tempFilename); err != nil && !errors.Is(err, os.ErrNotExist) {
			slog.Error("failed to remove temp file", "path", tempFilename, "err", err)
		}
	}

	if _, err := io.Copy(dst, src); err != nil {
		clean()
		return "", nil, fmt.Errorf("writing temp file: %w", err)
	}

	// maybe dst.Sync() for more reliability
	if err := dst.Close(); err != nil {
		clean()
		return "", nil, fmt.Errorf("closing temp file: %w", err)
	}

	return tempFilename, clean, nil
}
