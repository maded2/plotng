package main

import (
	"flag"

	"plotng/internal"
)

func main() {
	configFile := flag.String("config", "", "configuration file")
	address := flag.String("address", "", "local address to bind to, default any")
	port := flag.Int("port", 8484, "host server port number, default: 8484")

	flag.Parse()
	if flag.Parsed() == false || (len(*configFile) == 0) {
		flag.Usage()
		return
	}
	server := &internal.Server{}
	server.ProcessLoop(*configFile, *address, *port)
}
