package main

import . "os"

func main() {
	Exit(1) // want "direct call to os.Exit in main.main is forbidden"
}
