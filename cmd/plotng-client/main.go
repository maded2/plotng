package main

import (
	"flag"

	"plotng/internal"
)

func main() {
	host := flag.String("host", "localhost", "host server name, default: localhost")

	flag.Parse()
	if flag.Parsed() == false {
		flag.Usage()
		return
	}
	client := &internal.Client{}
	client.ProcessLoop(*host)
}
