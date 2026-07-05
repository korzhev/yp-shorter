package config

import (
	"flag"
)

const ShortLinkCharset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

var FlagShortLinkLength int
var FlagRunAddr string
var FlagBaseResultAddr string

func ParseFlags() {
	flag.StringVar(&FlagRunAddr, "a", ":8080", "address and port to run server")
	flag.StringVar(&FlagBaseResultAddr, "b", "http://localhost:8080", "base url for short link")
	flag.IntVar(&FlagShortLinkLength, "l", 6, "short link id length")
	flag.Parse()
}
