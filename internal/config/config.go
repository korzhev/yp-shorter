package config

import (
	"flag"

	"log"

	"github.com/caarlos0/env/v6"
)

const DefaultCharset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

type Config struct {
	ShortLinkLength  int    `env:"SH_LENGTH"`
	RunAddr          string `env:"SERVER_ADDRESS"`
	BaseResultAddr   string `env:"BASE_URL"`
	ShortLinkCharset string `env:"SH_CHARSET"`
	LogLevel         string `env:"LOG_LEVEL"`
}

var Conf Config

// test framework conflicts with init()
func ParseFlags() {
	flag.StringVar(&Conf.ShortLinkCharset, "c", DefaultCharset, "chars to use in id generator")
	flag.StringVar(&Conf.RunAddr, "a", ":8080", "address and port to run server")
	flag.StringVar(&Conf.BaseResultAddr, "b", "http://localhost:8080", "base url for short link")
	flag.IntVar(&Conf.ShortLinkLength, "l", 6, "short link id length")
	flag.StringVar(&Conf.LogLevel, "ll", "info", "log level")
	flag.Parse()

	err := env.Parse(&Conf)
	if err != nil {
		log.Fatal(err)
	}
}
