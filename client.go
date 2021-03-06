package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	"github.com/miska12345/MiskaRFS/src/host"
	msg "github.com/miska12345/MiskaRFS/src/message"
	"github.com/miska12345/MiskaRFS/src/tcp2"
)

func main() {
	// This program connect to a remote host by name
	c1, err := tcp2.ConnectToTCPServer("localhost:8080", "", "pc-admin")
	if err != nil {
		fmt.Println(err)
		return
	}
	for {
		data, _ := c1.Receive()
		if bytes.Equal(data, []byte("ok")) {
			break
		}
	}

	// Run ls remotely
	// Client-server communication is always in packet
	h := host.Request{
		Type: "text/cmd",
		Body: "ls",
	}

	bs, err := json.Marshal(h)
	if err != nil {
		fmt.Println(err)
		return
	}

	c1.Send(bs)
	fmt.Println("Waiting for response")
	d, err := c1.Receive()

	bsf, err := msg.ConvertFromNetForm(d)
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println(bsf.Msg)
	time.Sleep(time.Second)
}
