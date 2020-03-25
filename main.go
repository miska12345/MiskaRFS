package main

import (
	"fmt"

	"github.com/miska12345/MiskaRFS/src/fs"
	"github.com/miska12345/MiskaRFS/src/host"
)

func main() {
	_, err := fs.Init("src", []string{}, false)
	if err != nil {
		fmt.Println(err)
		return
	}
	_, err = host.Run(&host.ModuleConfig{
		Name:           "admin",
		BaseDir:        "src",
		InvisibleFiles: []string{"tcp2"},
		ReadOnly:       false,
		AddFeatures: map[string]func(args ...string) string{
			"version": func(args ...string) string {
				return "v1.0"
			},
		},
	})

	if err != nil {
		fmt.Println(err)
		return
	}
	for {

	}
}
