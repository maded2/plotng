package main

import (
	"flag"
	"plotng/internal"
)

func main() {
	configFile := flag.String("config", "", "configuration file")
	ui := flag.Bool("ui", false, "launch UI client only, it will attempt to connect to server")
	host := flag.String("host", "localhost", "host server name, default: localhost")
	port := flag.Int("port", 8484, "host server port number, default: 8484")

	flag.Parse()
	if flag.Parsed() == false || len(*configFile) == 0 {
		flag.Usage()
		return
	}
	if *ui {
		client := &internal.Client{}
		client.ProcessLoop(*host, *port)
	} else {
		server := &internal.Server{}
		server.ProcessLoop(*configFile, *port)
	}
}
