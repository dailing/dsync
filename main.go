package main

import (
	"flag"
	watcher "github.com/dailing/dsync/fsnotify"
	"github.com/dailing/levlog"
	"os"
	"os/signal"
	"syscall"
	"github.com/dailing/dsync/fsstatus"
)

// TODO make sql table to record files
// file path, local,abs ...
// file hash, part hash ...
// altered time.
// Use github.com/monmohan/xferspdy to get binary patch sys.

const DB_NAME = "FsStatus.db"


func main() {
	var listenPort int64
	var connectAddr string
	flag.Int64Var(&listenPort, "port", -1,
		"This field should be set if the server has a public ip addr. The port that this program listens to.")
	flag.StringVar(&connectAddr, "connect_to", "127.0.0.1:7222",
		"The address that this program connects to.")

	// TODO test this
	fss := fsstatus.NewFileStatus(DB_NAME)
	levlog.E(fss.CreateRootEntry(`C:\User\d\go`))
	levlog.E(fss.CreateOrUpdate(`C:\User\d\go\adf\sfda`))
	levlog.Trace(fss)
	return

	w, err := watcher.NewRecWatcher()
	levlog.F(err)
	if w == nil {
		levlog.Fatal("Error creating Watcher")
	}
	levlog.Info(w)
	levlog.F(w.Add(`.`))

	events := w.Events
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT)
	for {
		select {
		case s := <-sig:
			levlog.Info("Exiting...")
			levlog.Info(s.String())
			w.Close()
			return
		case e := <-events:
			levlog.Info(e.Name)
			levlog.Info(e.Op)
		case err := <-w.Errors:
			levlog.Error(err)
		}
	}
}
