// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build !plan9

// Package fsnotify provides a platform-independent interface for file system notifications.
package fsnotify

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/dailing/levlog"
	"os"
	"path/filepath"
)

// Event represents a single file system notification.
type Event struct {
	Name string // Relative path to the file or directory.
	Op   Op     // File operation that triggered the event.
}

// Op describes a set of file operations.
type Op uint32

// These are the generalized file operations that can trigger a notification.
const (
	Create Op = 1 << iota
	Write
	Remove
	Rename
	Chmod
)

func (op Op) String() string {
	// Use a buffer for efficient string concatenation
	var buffer bytes.Buffer

	if op&Create == Create {
		buffer.WriteString("|CREATE")
	}
	if op&Remove == Remove {
		buffer.WriteString("|REMOVE")
	}
	if op&Write == Write {
		buffer.WriteString("|WRITE")
	}
	if op&Rename == Rename {
		buffer.WriteString("|RENAME")
	}
	if op&Chmod == Chmod {
		buffer.WriteString("|CHMOD")
	}
	if buffer.Len() == 0 {
		return ""
	}
	return buffer.String()[1:] // Strip leading pipe
}

// String returns a string representation of the event in the form
// "file: REMOVE|WRITE|..."
func (e Event) String() string {
	return fmt.Sprintf("%q: %s", e.Name, e.Op.String())
}

// add rec add
type RecWatcher struct {
	watcher *Watcher
	Events  chan Event
	Errors  chan error
}

func (rw *RecWatcher) Add(name string) error {
	fileList := make([]string, 0)
	name, err := filepath.Abs(name)
	if err != nil {
		return err
	}
	err = filepath.Walk(name, func(path string, f os.FileInfo, err error) error {
		if err != nil {
			levlog.E(err)
			return err
		}
		if !f.IsDir() {
			return nil
		}
		levlog.Info("Adding path:", path)
		fileList = append(fileList, path)
		return nil
	})

	if err != nil {
		levlog.E(err)
		return err
	}
	for _, fileName := range fileList {
		if err := rw.watcher.Add(fileName); err != nil {
			levlog.E(err)
			return err
		}
	}
	return nil
}

func (rw *RecWatcher) Close() error {
	return rw.Close()
}

func (rw *RecWatcher) Remove(name string) error {
	name, err := filepath.Abs(name)
	if err != nil {
		return err
	}
	fileList := make([]string, 0)
	err = filepath.Walk(name, func(path string, f os.FileInfo, err error) error {
		if !f.IsDir() {
			return nil
		}
		levlog.Info("Remove path:", path)
		fileList = append(fileList, path)
		return nil
	})

	if err != nil {
		levlog.E(err)
		return err
	}
	for _, fileName := range fileList {
		if err := rw.watcher.Remove(fileName); err != nil {
			levlog.E(err)
			return err
		}
	}
	return nil
}

func NewRecWatcher() (*RecWatcher, error) {
	watcher, err := NewWatcher()
	return &RecWatcher{
		watcher: watcher,
		Events:  watcher.Events,
		Errors:  watcher.Errors,
	}, err
}

// Common errors that can be reported by a watcher
var ErrEventOverflow = errors.New("fsnotify queue overflow")
