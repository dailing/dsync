package fileTransfer

import (
	"fmt"
	"github.com/dailing/levlog"
	"net"
)

type SimpleSocketTransfer struct {
}

func NewSocketTransfer(host string) *SimpleSocketTransfer {
	return &SimpleSocketTransfer{}
}

/*
	Msg define:
	start with "--start--\n"
	length \n
	filename \n
	--end head--
	content
	end with "--end--\n"
*/

func (sp *SimpleSocketTransfer) ServeAt(port int) {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		// handle error
	}
	for {
		conn, err := ln.Accept()
		if err != nil {
			// handle error
		}
		go handleConnection(conn)
	}
}

func (sp *SimpleSocketTransfer) ConnectTo() {
	conn, err := net.Dial("tcp", "127.0.0.1:8080")
	if err != nil {
		// handle error
	}
	handleConnection(conn)
}

func handleConnection(conn net.Conn) {
	defer conn.Close()
	buffer := make([]byte, 1024)
	for {
		conn.Write([]byte("FUCK!\n"))
		n, err := conn.Read(buffer)
		levlog.E(err)
		levlog.Info("Read NUM:", n)
		levlog.Info("Read CON:", string(buffer))
		if n < cap(buffer) {
			break
		}
	}
}
