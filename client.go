package main

import (
	"bytes"
	"fmt"
	"os"

	"github.com/miska12345/MiskaRFS/src/tcp"
)

func main() {
	c1, _, _, err := tcp.ConnectToTCPServer("localhost:8080", "", "what", os.Args[1])
	if err != nil {
		fmt.Println(err)
		return
	}
	if os.Args[1] == os.Args[2] {
		var data []byte
		for {
			data, err = c1.Receive()
			if bytes.Equal(data, []byte{1}) {
				continue
			}
			break
		}
		fmt.Println(string(data))
	} else {
		c1.Send([]byte("hello pc!"))
	}
}
