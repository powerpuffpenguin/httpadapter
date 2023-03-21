package main

import "log"

func main() {
	log.SetFlags(log.Lshortfile | log.LstdFlags)
	root := root()
	root.AddCommand(
		server(),
		tunnel(),
		client(),
		echo(),
	)
	root.Execute()
}
