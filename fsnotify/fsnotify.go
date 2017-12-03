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

// Called when a event happened, if return True, then continue process this event
// If return False, ignore this event.
type EventFilter interface {
	Filter(events Event) bool
}

// Called when add or remove dir to watch list
type ListAlterHook interface {
	OnAddFile(name string, isDir bool)
	OnRemoveFile(name string, isDir bool)
}

// RecWatcher
type RecWatcher struct {
	watcher  *Watcher
	Events   chan Event
	Errors   chan error
	filters  []EventFilter
	hooks    []ListAlterHook
	watchMap map[string]bool
	stop     chan struct{}
}

func NewRecWatcher() (*RecWatcher, error) {
	watcher, err := NewWatcher()
	levlog.E(err)

	recWatcher := &RecWatcher{
		watcher:  watcher,
		Events:   make(chan Event, 50),
		Errors:   watcher.Errors,
		filters:  make([]EventFilter, 0),
		hooks:    make([]ListAlterHook, 0),
		watchMap: make(map[string]bool),
		stop:     make(chan struct{}, 1),
	}
	recWatcher.AddFilter(recWatcher)
	recWatcher.AddHook(recWatcher)
	go recWatcher.handleEvents()
	return recWatcher, err
}

// this should be run in go-routine
func (rw *RecWatcher) handleEvents() {
	stop := false
	for {
		if stop {
			break
		}
		select {
		case event := <-rw.watcher.Events:
			pass := true
			for _, f := range rw.filters {
				if !f.Filter(event) {
					pass = false
					break
				}
			}
			if pass {
				select {
				case <-rw.stop:
					stop = true
					break
				case rw.Events <- event:
					break
				}
			}
		case <-rw.stop:
			stop = true
			break
		}
	}
}

// Fix new sub dir problem
func (rw *RecWatcher) Filter(event Event) bool {
	if ok, _ := rw.watchMap[event.Name]; ok {
		if event.Op&Remove == Remove {
			rw.Remove(event.Name)
			return true
		}
	} else {
		if event.Op&Create == Create {
			state, err := os.Stat(event.Name)
			if err != nil {
				levlog.E(err)
				return true
			}
			if state.IsDir() {
				rw.Add(event.Name)
			}
		}
	}
	return true
}

func (rw *RecWatcher) OnAddFile(name string, isDir bool) {
	if isDir {
		rw.watchMap[name] = true
	}
}

func (rw *RecWatcher) OnRemoveFile(name string, isDir bool) {
	if isDir {
		delete(rw.watchMap, name)
	}
}

func (rw *RecWatcher) AddFilter(filter EventFilter) {
	rw.filters = append(rw.filters, filter)
}

func (rw *RecWatcher) AddHook(hook ListAlterHook) {
	if hook != nil {
		rw.hooks = append(rw.hooks, hook)
	}
}

func (rw *RecWatcher) Add(name string) error {
	fileList := make([]string, 0)
	name, err := filepath.Abs(name)
	if err != nil {
		return err
	}

	fi, err := os.Stat(name)
	if err != nil {
		levlog.E(err)
		return err
	}
	if !fi.IsDir() {
		rw.watcher.Add(name)
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
	defer close(rw.Events)
	rw.stop <- struct{}{}
	levlog.Info("sent stop signal")
	return rw.watcher.Close()
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

// Common errors that can be reported by a watcher
var ErrEventOverflow = errors.New("fsnotify queue overflow")
