package fileTransfer

import (
	"bufio"
	"bytes"
	"fmt"
	"github.com/dailing/levlog"
	"io"
	"net"
	"strconv"
)

type Message struct {
	Cmd      string
	FileName string
	Payload  []byte
}

type SimpleSocketTransfer struct {
}

func NewSocketTransfer(host string) *SimpleSocketTransfer {
	return &SimpleSocketTransfer{}
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
		go MassageHandler(conn, msg)
	}
}

func (sp *SimpleSocketTransfer) ConnectTo(inMsg, outMsg chan *Message) {
	conn, err := net.Dial("tcp", "127.0.0.1:18080")
	if err != nil {
		levlog.E(err)
		return
	}
	levlog.Info("Connectiong to ", conn.RemoteAddr().String())
	go MassageHandler(conn, outMsg)
	go sendConn(inMsg, conn)
}

func sendConn(chanstr chan *Message, conn net.Conn) {
	w := bufio.NewWriter(conn)
	for {
		str := <-chanstr
		levlog.Trace("get ", str)

		w.WriteString("--start--\n")                         // start of message
		w.WriteString(fmt.Sprintf("%s\n", str.Cmd))          // cmd
		w.WriteString(fmt.Sprintf("%d\n", len(str.Payload))) // length
		w.WriteString(fmt.Sprintf("%s\n", str.FileName))     // file name
		w.WriteString("--end head--\n")                      // head end
		w.Write(str.Payload)                                 // payload
		w.WriteString("--end--\n")                           // end of message
		levlog.E(w.Flush())
	}
}

func MassageHandler(conn io.Reader, msg chan *Message) {
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
				status = IDLE
				break
			}
			status = IDLE
			// now the
			levlog.Trace("Read a file :", fileName)
			levlog.Trace("File Len    :", length)
			levlog.Trace("Cmd         :", cmd)
			msg <- &Message{
				Cmd:      cmd,
				FileName: fileName,
				Payload:  payloadbuf.Bytes(),
			}
		}
	}
}
