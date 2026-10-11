package main

import system "os"

func main() {
	system.Exit(1) // want "direct call to os.Exit in main.main is forbidden"
}
