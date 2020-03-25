package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/miska12345/MiskaRFS/src/host"
	"github.com/miska12345/MiskaRFS/src/tcp2"
)

func main() {
	c1, err := tcp2.ConnectToTCPServer("localhost:8080", "", "admin")
	if err != nil {
		fmt.Println(err)
		return
	}
	for {
		data, _ := c1.Receive()
		if bytes.Equal(data, []byte("ok")) {
			log.Print("ok")
			break
		}
	}

	h := host.Request{
		Type: "text/cmd",
		Body: "rm deleteMe",
	}

	bs, err := json.Marshal(h)
	if err != nil {
		fmt.Println(err)
		return
	}

	c1.Send(bs)
	fmt.Println("Waiting for response")
	d, err := c1.Receive()
	fmt.Println(string(d))

	time.Sleep(time.Second)
}
