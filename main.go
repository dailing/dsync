package main

import (
	"flag"
	watcher "github.com/dailing/dsync/fsnotify"
	"github.com/dailing/levlog"
	"time"
)

func main() {
	var listenPort int64
	var connectAddr string
	flag.Int64Var(&listenPort, "port", -1,
		"This field should be set if the server has a public ip addr. The port that this program listens to.")
	flag.StringVar(&connectAddr, "connect_to", "127.0.0.1:7222",
		"The address that this program connects to.")

	w, err := watcher.NewRecWatcher()
	levlog.F(err)
	if w == nil {
		levlog.Fatal("Error creating Watcher")
	}
	levlog.Info(w)
	levlog.F(w.Add(`.`))
	//levlog.F(w.Remove(`.`))

	events := w.Events
	done := make(chan int, 1)
	go func() {
		time.Sleep(time.Second * 1)
		levlog.Info("Closing")
		w.Close()
		done <- 0
	}()
	for {
		select {
		case <-done:
			break
		case e := <-events:
			levlog.Info(e.Name)
			levlog.Info(e.Op)
		case err := <-w.Errors:
			levlog.Error(err)
		}
	}
}
