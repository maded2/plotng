package main

import (
	"flag"
	"plotng/internal"
)

func main() {
	configFile := flag.String("config", "", "configuration file")
	ui := flag.Bool("ui", false, "launch UI client only, it will attempt to connect to server")
	flag.Parse()
	if flag.Parsed() == false || len(*configFile) == 0 {
		flag.Usage()
		return
	}
	if *ui {
		client := &internal.Client{}
		client.ProcessLoop()
	} else {
		server := &internal.Server{}
		server.ProcessLoop(*configFile)
	}
}
