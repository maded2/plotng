package main

import (
	"flag"

	"plotng/internal"
)

func main() {
	hosts := flag.String("hosts", "localhost", "hosts to query, separated by comma, default: localhost")

	flag.Parse()
	if flag.Parsed() == false {
		flag.Usage()
		return
	}
	client := &internal.Client{}
	client.ProcessLoop(*hosts)
}
