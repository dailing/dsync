package fileTransfer

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"github.com/dailing/dsync/trigger"
	"github.com/dailing/levlog"
	"io"
	"net"
	"strconv"
)

const (
	RecvMsg = "__RECV_MSG__"
	SendMsg = "__SEND_MSG__"
)

type Message struct {
	Cmd      string
	FileName string
	Payload  []byte
}

type SimpleSocketTransfer struct {
	host string
	T    trigger.Trigger
}

func NewSocketTransfer(host string) *SimpleSocketTransfer {
	return &SimpleSocketTransfer{
		host: host,
		T:    trigger.New(),
	}
}

/*
	Msg define:
	"--start--\n"
	"Cmd\n"
	length \n
	filename \n
	--end head--
	content
	end with "--end--\n"
*/

func (sp *SimpleSocketTransfer) ServeAt(port int) {
	ln, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", port))
	if err != nil {
		levlog.F(err)
	}
	levlog.Info("Listen at:", ln.Addr().String())
	msg := make(chan *Message)
	go func() {
		for {
			m := <-msg
			levlog.Info(*m)
		}
	}()
	for {
		conn, err := ln.Accept()
		if err != nil {
			levlog.E(err)
		}
		levlog.Info("Connection from ", conn.RemoteAddr().String())
		go sp.MassageHandler(conn)
		sp.T.On(SendMsg, sp.sendConn(conn))
	}
}

func (sp *SimpleSocketTransfer) ConnectTo() {
	conn, err := net.Dial("tcp", "127.0.0.1:18080")
	if err != nil {
		levlog.E(err)
		return
	}
	levlog.Info("Connectiong to ", conn.RemoteAddr().String())
	go sp.MassageHandler(conn)
	sp.T.On(SendMsg, sp.sendConn(conn))
}

func (sp *SimpleSocketTransfer) sendConn(conn net.Conn) func(str *Message) {
	levlog.Info("registering function")
	return func(str *Message) {
		levlog.Info("receive message sending event")
		w := bufio.NewWriter(conn)
		var write = func(s string) error {
			i, err := w.WriteString(s)
			if i != len(s) {
				levlog.Error("Size not correct whild sending ", s)
				return errors.New("Size not correct while sending " + s)
			}
			levlog.E(err)
			return err
		}
		levlog.E(write("--start--\n"))                         // start of message
		levlog.E(write(fmt.Sprintf("%s\n", str.Cmd)))          // cmd
		levlog.E(write(fmt.Sprintf("%d\n", len(str.Payload)))) // length
		levlog.E(write(fmt.Sprintf("%s\n", str.FileName)))     // file name
		levlog.E(write("--end head--\n"))                      // head end
		levlog.E(write(string(str.Payload)))                   // payload
		levlog.E(write("--end--\n"))                           // end of message
		levlog.E(w.Flush())
	}
}

func (sp *SimpleSocketTransfer) MassageHandler(conn io.Reader) {
	const (
		IDLE     = iota
		START
		CMD
		LENGTH
		FILENAME
		HEADEND
		PAYLOAD
		ERROR
	)
	defer levlog.Info("Exiting MassageHandler")
	rw := bufio.NewReader(conn)
	status := IDLE
	var (
		length     int64
		fileName   string
		cmd        string
		payloadbuf *bytes.Buffer
	)
	for status != ERROR {
		levlog.Info(status)
		switch status {
		case IDLE:
			b, prefix, err := rw.ReadLine()
			levlog.E(err)
			if err != nil {
				status = ERROR
				break
			}
			if prefix || string(b) != "--start--" {
				status = IDLE
				break
			}
			status = START
		case START:
			b, prefix, err := rw.ReadLine()
			levlog.E(err)
			if prefix || err != nil {
				status = IDLE
				break
			}
			cmd = string(b)
			status = CMD
		case CMD:
			b, prefix, err := rw.ReadLine()
			if prefix || err != nil {
				levlog.E(err)
				status = IDLE
				break
			}
			s := string(b)
			length, err = strconv.ParseInt(s, 10, 64)
			if err != nil {
				status = IDLE
				levlog.E(err)
				break
			}
			status = LENGTH
		case LENGTH:
			b, prefix, err := rw.ReadLine()
			if err != nil || prefix {
				levlog.E(err)
				status = IDLE
			}
			fileName = string(b)
			status = FILENAME
		case FILENAME:
			b, prefix, err := rw.ReadLine()
			levlog.E(err)
			if prefix || string(b) != "--end head--" || err != nil {
				status = IDLE
				break
			}
			status = HEADEND
			payloadbuf = bytes.NewBuffer(make([]byte, 0))
			payloadbuf.Reset()
		case HEADEND:
			l := length - int64(payloadbuf.Len())
			if l > 1024 {
				l = 1024
			}
			buf := make([]byte, l)
			readLen, err := rw.Read(buf)
			if err != nil {
				levlog.E(err)
				status = IDLE
				break
			}
			_, err = payloadbuf.Write(buf[:readLen])
			if err != nil {
				levlog.E(err)
				status = IDLE
				break
			}
			if payloadbuf.Len() >= int(length) {
				status = PAYLOAD
			}
		case PAYLOAD:
			b, prefix, err := rw.ReadLine()
			levlog.E(err)
			if prefix || string(b) != "--end--" || err != nil {
				levlog.Error("error read str")
				status = ERROR
				break
			}
			status = IDLE
			// now the
			levlog.Trace("Read a file :", fileName)
			levlog.Trace("File Len    :", length)
			levlog.Trace("Cmd         :", cmd)
			levlog.Info("Firing msg")
			sp.T.Fire(RecvMsg, &Message{
				Cmd:      cmd,
				FileName: fileName,
				Payload:  payloadbuf.Bytes(),
			})
		}
	}
}
