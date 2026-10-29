package main

import "os"

func main() {
	os.Exit(0) // want "direct call to os.Exit in main.main is forbidden"
	if len(os.Args) > 1 {
		os.Exit(1) // want "direct call to os.Exit in main.main is forbidden"
	}
	for range 1 {
		os.Exit(2) // want "direct call to os.Exit in main.main is forbidden"
	}
	defer os.Exit(3) // want "direct call to os.Exit in main.main is forbidden"
	go os.Exit(4)    // want "direct call to os.Exit in main.main is forbidden"
	(os.Exit)(5)     // want "direct call to os.Exit in main.main is forbidden"
}
