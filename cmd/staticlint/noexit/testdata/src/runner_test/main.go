package main

import "os"

// A normal package ending in _test must still be checked.
func main() {
	os.Exit(1) // want "direct call to os.Exit in main.main is forbidden"
}
