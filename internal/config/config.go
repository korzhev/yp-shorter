package config

import (
	"flag"
)

type Config struct {
	FlagShortLinkLength  int
	FlagRunAddr          string
	FlagBaseResultAddr   string
	FlagShortLinkCharset string
}

var Conf Config

func init() {
	flag.StringVar(&Conf.FlagShortLinkCharset, "c", "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789", "chars to use in id generator")
	flag.StringVar(&Conf.FlagRunAddr, "a", ":8080", "address and port to run server")
	flag.StringVar(&Conf.FlagBaseResultAddr, "b", "http://localhost:8080", "base url for short link")
	flag.IntVar(&Conf.FlagShortLinkLength, "l", 6, "short link id length")
	flag.Parse()
}
