package main

import (
	"flag"

	"plotng/internal"
)

func main() {
	hosts := flag.String("hosts", "localhost", "hosts to query, separated by comma, default: localhost")
	alternateMouse := flag.Bool("alternate-mouse", false, "use alternate mouse setup (for PuTTy)")

	flag.Parse()
	if flag.Parsed() == false {
		flag.Usage()
		return
	}
	client := &internal.Client{
		AlternateMouse: *alternateMouse,
	}
	client.ProcessLoop(*hosts)
}
