package main

import (
	"fmt"

	"github.com/miska12345/MiskaRFS/src/fs"
	"github.com/miska12345/MiskaRFS/src/host"
	msg "github.com/miska12345/MiskaRFS/src/message"
)

func main() {
	// This program will make this PC accessible via internet

	// Iniotialize the file system
	_, err := fs.Init("src", []string{}, false)
	if err != nil {
		fmt.Println(err)
		return
	}

	// Run the host with config
	_, err = host.Run(&host.ModuleConfig{
		Name:           "pc-admin",
		BaseDir:        "src",
		InvisibleFiles: []string{"tcp2"},
		ReadOnly:       false,
		AddFeatures: map[string]func(args ...string) *msg.Message{
			"version": func(args ...string) *msg.Message {
				return msg.New(msg.TYPE_RESPONSE, "v1.0")
			},
		},
	})

	if err != nil {
		fmt.Println(err)
		return
	}
	forever := make(chan bool)
	<-forever
}
