package main

import (
	"flag"
	watcher "github.com/dailing/dsync/fsnotify"
	"github.com/dailing/levlog"
)

type Sender interface {
	send(file string) error
}

type EventFilter interface {
	Filter(events chan watcher.Event) chan watcher.Event
}

type IDEFilter struct{}

func (IDEFilter) Filter(events chan watcher.Event) chan watcher.Event {
	return events
}


func main() {
	var listenPort int64
	var connectAddr string
	flag.Int64Var(&listenPort, "port", -1,
		"This field should be set if the server has a public ip addr. The port that this program listens to.")
	flag.StringVar(&connectAddr, "connect_to", "127.0.0.1:7222",
		"The address that this program connects to.")

	var filter EventFilter
	filter = IDEFilter{}

	w, err := watcher.NewRecWatcher()
	levlog.F(err)
	if w == nil {
		levlog.Fatal("Error creating Watcher")
	}
	levlog.Info(w)
	levlog.F(w.Add(`..`))
	levlog.F(w.Remove(`.`))
	//levlog.F(w.Remove(`C:\Users\dailing\go\pkg`))
	//levlog.F(w.Remove(`C:\Users\dailing\go\src`))

	events := filter.Filter(w.Events)
	for {
		select {
		case e := <-events:
			levlog.Info(e.Name)
			levlog.Info(e.Op)
		case err := <-w.Errors:
			levlog.Error(err)
		}
	}
}
