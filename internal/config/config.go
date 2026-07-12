package config

import (
	"flag"
)

const DefaultCharset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

type Config struct {
	ShortLinkLength  int
	RunAddr          string
	BaseResultAddr   string
	ShortLinkCharset string
}

var Conf Config

// test framework conflicts with init()
func ParseFlags() {
	flag.StringVar(&Conf.ShortLinkCharset, "c", DefaultCharset, "chars to use in id generator")
	flag.StringVar(&Conf.RunAddr, "a", ":8080", "address and port to run server")
	flag.StringVar(&Conf.BaseResultAddr, "b", "http://localhost:8080", "base url for short link")
	flag.IntVar(&Conf.ShortLinkLength, "l", 6, "short link id length")
	flag.Parse()
}
