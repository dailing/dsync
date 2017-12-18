// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build !plan9

package fsnotify

import (
	"math/rand"
	"testing"
	"time"
)

func TestEvent_Combine(t *testing.T) {
	e1 := Event{
		Name: "a",
		Op:   Create,
	}
	e1.Combine(Event{
		Name: "a",
		Op:   Write,
	})
	if e1.Op != Create|Write {
		t.Errorf("Combine Not Currect, expect %s, Got %d",
			(Create | Write).String(),
			e1.Op.String())
	}
}

func TestPriorityQueue_Len(t *testing.T) {
	var pq = NewPriorityQueue()
	now := time.Now()
	for i := 0; i < 100; i++ {
		pq.PushItem(Item{
			priority: now.Add(time.Duration(rand.Int31())),
		})
	}
	prevItem := pq.PopItem()
	for pq.Len() > 0 {
		itemt := pq.Front()
		item := pq.PopItem()
		if itemt.priority != item.priority {
			t.Errorf("Error: priority queue ")
		}
		if prevItem.priority.After(item.priority) {
			t.Errorf("Error: priority queue ")
		}
	}
}

func TestEventStringWithValue(t *testing.T) {
	for opMask, expectedString := range map[Op]string{
		Chmod | Create: `"/usr/someFile": CREATE|CHMOD`,
		Rename:         `"/usr/someFile": RENAME`,
		Remove:         `"/usr/someFile": REMOVE`,
		Write | Chmod:  `"/usr/someFile": WRITE|CHMOD`,
	} {
		event := Event{Name: "/usr/someFile", Op: opMask}
		if event.String() != expectedString {
			t.Fatalf("Expected %s, got: %v", expectedString, event.String())
		}

	}
}

func TestEventOpStringWithValue(t *testing.T) {
	expectedOpString := "WRITE|CHMOD"
	event := Event{Name: "someFile", Op: Write | Chmod}
	if event.Op.String() != expectedOpString {
		t.Fatalf("Expected %s, got: %v", expectedOpString, event.Op.String())
	}
}

func TestEventOpStringWithNoValue(t *testing.T) {
	expectedOpString := ""
	event := Event{Name: "testFile", Op: 0}
	if event.Op.String() != expectedOpString {
		t.Fatalf("Expected %s, got: %v", expectedOpString, event.Op.String())
	}
}
