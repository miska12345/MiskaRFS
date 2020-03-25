package tcp2

import (
	"bytes"
	"testing"
	"time"

	"github.com/miska12345/MiskaRFS/src/tcp2"

	"github.com/stretchr/testify/assert"
)

func TestTCP(t *testing.T) {
	go tcp2.Run("8080", "debug", "")
	time.Sleep(2 * time.Second)
	c1, err := tcp2.ConnectToTCPServer("localhost:8080", "", "debug")

	assert.Nil(t, err)

	c2, err := tcp2.ConnectToTCPServer("localhost:8080", "", "debug")

	assert.Nil(t, err)

	go func() {
		for {
			var data []byte
			data, err = c1.Receive()
			assert.Nil(t, err)
			if bytes.Equal(data, []byte{1}) {
				continue
			} else {
				assert.Equal(t, []byte("Hello, World!"), data)
			}
		}
	}()

	for {
		var data []byte
		data, err = c2.Receive()
		assert.Nil(t, err)
		if bytes.Equal(data, []byte("ok")) {
			break
		}
	}
	err = c2.Send([]byte("Hello, World!"))
	assert.Nil(t, err)
	time.Sleep(1 * time.Second)
	c2.Close()

	c2, err = tcp2.ConnectToTCPServer("localhost:8080", "", "debug")
	assert.Nil(t, err)
	err = c2.Send([]byte("Hello, World!"))
	assert.Nil(t, err)
	time.Sleep(1 * time.Second)
	c2.Close()
}
