package main

import (
	"bytes"
	"fmt"
	"time"

	log "github.com/miska12345/MiskaRFS/src/logger"
	"github.com/miska12345/MiskaRFS/src/tcp2"
)

func main() {
	c1, _ := tcp2.ConnectToTCPServer("localhost:8080", "", "admin")
	c2, _ := tcp2.ConnectToTCPServer("localhost:8080", "", "admin")

	go func() {
		for {
			data, _ := c1.Receive()
			if bytes.Equal(data, []byte{1}) {
				continue
			}
			fmt.Println(string(data))
			c1.Send(data)
		}
	}()
	for {
		data, _ := c2.Receive()
		if bytes.Equal(data, []byte("ok")) {
			break
		}
	}
	c2.Send([]byte("hello"))
	data, _ := c2.Receive()
	if bytes.Equal(data, []byte("hello")) {
		log.Debug("good")
	}

	time.Sleep(time.Second)
	c2.Close()
	c2, _ = tcp2.ConnectToTCPServer("localhost:8080", "", "admin")
	for {
		data, _ := c2.Receive()
		if bytes.Equal(data, []byte("ok")) {
			break
		}
	}
	c2.Send([]byte("hello"))
	data, _ = c2.Receive()
	if bytes.Equal(data, []byte("hello")) {
		log.Debug("good")
	}
	time.Sleep(time.Second)
	c2.Close()
}
