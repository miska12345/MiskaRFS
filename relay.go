package main

import "github.com/miska12345/MiskaRFS/src/tcp2"

func main() {
	// This represents the relay
	tcp2.Run("8080", "debug", "")

	forever := make(chan bool)
	<-forever
}
