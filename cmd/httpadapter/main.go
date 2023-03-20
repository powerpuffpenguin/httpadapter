package main

func main() {
	root := root()
	root.AddCommand(
		server(),
		tunnel(),
	)
	root.Execute()
}
