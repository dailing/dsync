// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build !plan9

// Package fsnotify provides a platform-independent interface for file system notifications.
package fsnotify

import (
	"bytes"
	"container/heap"
	"errors"
	"fmt"
	"github.com/dailing/levlog"
	"github.com/ryanuber/go-glob"
	"os"
	"path/filepath"
	"time"
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

func (e *Event) Combine(e2 Event) {
	e.Op |= e2.Op
}

// Filter the events
type EventFilter interface {
	Filter(*chan Event) *chan Event
}

// Define the delayed filter
type Item struct {
	value    string
	priority time.Time
	index    int // The index of the item in the heap.
}

// A PriorityQueue implements heap.Interface and holds Items.
type PriorityQueue []*Item

func NewPriorityQueue() *PriorityQueue {
	pq := make(PriorityQueue, 0)
	heap.Init(&pq)
	return &pq
}
func (pq PriorityQueue) Len() int { return len(pq) }

func (pq PriorityQueue) Less(i, j int) bool {
	return pq[i].priority.Before(pq[j].priority)
}

func (pq PriorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}

func (pq *PriorityQueue) Push(x interface{}) {
	n := len(*pq)
	item := x.(*Item)
	item.index = n
	*pq = append(*pq, item)
}
func (pq *PriorityQueue) PushItem(x Item) {
	heap.Push(pq, &x)
}

func (pq *PriorityQueue) PopItem() Item {
	item := heap.Pop(pq).(*Item)
	return *item
}

func (pq *PriorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	item.index = -1 // for safety
	*pq = old[0 : n-1]
	return item
}

// update modifies the priority and value of an Item in the queue.
func (pq *PriorityQueue) update(item *Item, value string, priority time.Time) {
	item.value = value
	item.priority = priority
	heap.Fix(pq, item.index)
}

func (pq *PriorityQueue) Front() *Item {
	return (*pq)[0]
}

type DelayedFilter struct {
	delayTime     time.Duration
	stalledEvents map[string]Event
	issueTime     map[string]time.Time
	pq            *PriorityQueue
	fireEvent     chan string
}

func NewDeleyedFilter(duration time.Duration) *DelayedFilter {
	levlog.Trace("Create Delayed Filter")
	df := &DelayedFilter{
		delayTime:     duration,
		stalledEvents: make(map[string]Event),
		issueTime:     make(map[string]time.Time),
		pq:            NewPriorityQueue(),
		fireEvent:     make(chan string, 5),
	}
	return df
}

type minTimer struct {
	timer              *time.Timer
	currentTimeOutTime time.Time
}

func (t *minTimer) after(d time.Duration) {
	newTimeOut := time.Now().Add(d)
	if t.currentTimeOutTime.After(newTimeOut) || t.currentTimeOutTime.Before(time.Now()) {
		levlog.Trace("NewTime at", newTimeOut)
		t.timer.Stop()
		t.timer = time.NewTimer(d)
		t.currentTimeOutTime = newTimeOut
	}
}
func (t *minTimer) at(newTimeOut time.Time) {
	if t.currentTimeOutTime.After(newTimeOut) || t.currentTimeOutTime.Before(time.Now()) {
		levlog.Trace("NewTime at", newTimeOut)
		t.timer.Stop()
		t.timer = time.NewTimer(newTimeOut.Sub(time.Now()))
		t.currentTimeOutTime = newTimeOut
	}
}

func (df *DelayedFilter) Filter(cevent *chan Event) *chan Event {
	levlog.Trace("Starting Delayed filter")
	newEventChan := make(chan Event)
	go func() {
		levlog.Trace("Running filter process")
		//currentTimeOutTime := time.Now().Add(time.Second * 10)
		timer := minTimer{
			timer:              time.NewTimer(time.Hour),
			currentTimeOutTime: time.Now().Add(time.Hour),
		}
		for {
			select {
			case event := <-*cevent:
				if event.Name == "" {
					levlog.Fatal("Empty Name")
				}
				levlog.Trace("Received %v", event)
				newEventAt := time.Now().Add(df.delayTime)
				df.pq.PushItem(Item{
					value:    event.Name,
					priority: newEventAt,
				})
				// set new issue time
				if lTime, ok := df.issueTime[event.Name]; !ok || newEventAt.After(lTime) {
					df.issueTime[event.Name] = newEventAt
				}
				// Combine the event Op code
				event.Combine(df.stalledEvents[event.Name])
				df.stalledEvents[event.Name] = event
				timer.after(df.delayTime)
				// TODO FIX THIS
				//levlog.Warning("VAL:", df.stalledEvents[event.Name])
			case <-timer.timer.C:
				cTime := time.Now()
				levlog.Trace("Timeout")
				// handle time event and read from pq, issue event when necessary
				issueList := make(map[string]bool, 10)
				for df.pq.Len() > 0 && !df.pq.Front().priority.After(cTime) {
					item := df.pq.PopItem()
					levlog.Tracef("Len %d, item %v", df.pq.Len(), item)
					if !df.issueTime[item.value].After(cTime) {
						issueList[item.value] = true
					}
				}
				for key, _ := range issueList {
					levlog.Tracef("DeleyedFilter, issue %s", df.stalledEvents[key])
					newEventChan <- df.stalledEvents[key]
					if _, ok := df.stalledEvents[key]; !ok {
						levlog.Error("Empty Name", key)
					}
					delete(df.stalledEvents, key)
					delete(df.issueTime, key)
				}
				if df.pq.Len() > 0 {
					levlog.Trace("Setting New timer")
					timer.at(df.pq.Front().priority)
					timer.after(df.delayTime)
				} else {
					// Nothing in queue, than no thing in the stalled map.
					timer.after(time.Hour)
				}
			}
		}
		levlog.Info("Exiting ......")
	}()
	return &newEventChan
}

type PatternFilter struct {
	ignorePatterns []string
	newPatternChan chan string
}

func NewPatternFilter() *PatternFilter {
	return &PatternFilter{
		ignorePatterns: make([]string, 0),
		newPatternChan: make(chan string),
	}
}

func (pf *PatternFilter) AddIgnore(s string) {
	select {
	case pf.newPatternChan <- s:
	default:
		pf.ignorePatterns = append(pf.ignorePatterns, s)
	}
}

func (pf *PatternFilter) Filter(cevent *chan Event) *chan Event {
	newEventChan := make(chan Event, 10)
	go func() {
		for {
			select {
			case event := <-*cevent:
				block := false
				for _, p := range pf.ignorePatterns {
					if glob.Glob(p, event.Name) {
						block = true
						break
					}
				}
				if !block {
					newEventChan <- event
				}
			}
		}
		levlog.Info("Exiting ......")
	}()
	return &newEventChan
}

// RecWatcher
type RecWatcher struct {
	watcher              *Watcher
	Events               chan Event
	Errors               chan error
	lastEventAfterFilter *chan Event
	watchMap             map[string]bool
	stop                 chan struct{}
}

func NewRecWatcher() (*RecWatcher, error) {
	watcher, err := NewWatcher()
	levlog.E(err)

	recWatcher := &RecWatcher{
		watcher:              watcher,
		Events:               make(chan Event, 50),
		Errors:               watcher.Errors,
		watchMap:             make(map[string]bool),
		stop:                 make(chan struct{}, 1),
		lastEventAfterFilter: &watcher.Events,
	}
	recWatcher.AddFilter(recWatcher)
	ignoreFilter := NewPatternFilter()
	ignoreFilter.AddIgnore("*.idea/*")
	ignoreFilter.AddIgnore("*.idea")
	ignoreFilter.AddIgnore("*.git")
	ignoreFilter.AddIgnore("*.git/*")
	ignoreFilter.AddIgnore("*.idea\\*")
	ignoreFilter.AddIgnore("*.git\\*")
	recWatcher.AddFilter(ignoreFilter)
	recWatcher.AddFilter(NewDeleyedFilter(time.Millisecond * 100))
	go recWatcher.handleEvents()
	return recWatcher, err
}

// this should be run in go-routine
func (rw *RecWatcher) handleEvents() {
	stop := false
	for !stop {
		select {
		case rw.Events <- <-*rw.lastEventAfterFilter:
		case <-rw.stop:
			stop = true
			break
		}
	}
}

// Fix new sub dir problem
func (rw *RecWatcher) Filter(eventChain *chan Event) *chan Event {
	levlog.Trace("Starting default filter")
	newChan := make(chan Event)
	go func() {
		for {
			event := <-*eventChain
			newChan <- event
			levlog.Trace("Default Filter, get event:  ", event)
			if ok, _ := rw.watchMap[event.Name]; ok {
				if event.Op&Remove == Remove {
					delete(rw.watchMap, event.Name)
					continue
				}
			} else {
				if event.Op&Create == Create {
					state, err := os.Stat(event.Name)
					if err != nil {
						levlog.E(err)
						continue
					}
					if state.IsDir() {
						rw.Add(event.Name)
					}
				}
			}
		}
		levlog.Info("Exiting .........")
	}()
	return &newChan
}

func (rw *RecWatcher) AddFilter(filter EventFilter) {
	rw.lastEventAfterFilter = filter.Filter(rw.lastEventAfterFilter)
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
		rw.watchMap[fileName] = true
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
		delete(rw.watchMap, fileName)
		if err := rw.watcher.Remove(fileName); err != nil {
			levlog.E(err)
			return err
		}
	}
	return nil
}

// Common errors that can be reported by a watcher
var ErrEventOverflow = errors.New("fsnotify queue overflow")
