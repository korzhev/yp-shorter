package main

import "os"

func helper() {
	os.Exit(1)
}

func init() {
	os.Exit(1)
}

type runner struct{}

func (runner) main() {
	os.Exit(1)
}

func main() {
	helper()
	exit := os.Exit
	exit(1)
	func() {
		os.Exit(1)
	}()
	f := func() {
		os.Exit(1)
	}
	f()
	os := struct{ Exit func(int) }{Exit: func(int) {}}
	os.Exit(1)
}
