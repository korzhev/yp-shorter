package config

import (
	"flag"
	"fmt"

	"github.com/caarlos0/env/v6"
)

const DefaultCharset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

type StorageType int

const (
	InMemory StorageType = iota
	File
	Database
)

type Config struct {
	ShortLinkLength  int    `env:"SH_LENGTH"`
	RunAddr          string `env:"SERVER_ADDRESS"`
	BaseResultAddr   string `env:"BASE_URL"`
	ShortLinkCharset string `env:"SH_CHARSET"`
	LogLevel         string `env:"LOG_LEVEL"`
	FileStoragePath  string `env:"FILE_STORAGE_PATH"`
	DBDSN            string `env:"DATABASE_DSN"`
	TokenExpMinutes  int    `env:"TOKEN_EXP_MINUTES"`
	TokenSecret      string `env:"TOKEN_SECRET"`
	StorageType      StorageType
}

func (c *Config) DetectStorageType() StorageType {
	c.StorageType = InMemory

	if c.FileStoragePath != "" {
		c.StorageType = File
	}

	if c.DBDSN != "" {
		c.StorageType = Database
	}
	return c.StorageType
}

var Conf Config

// test framework conflicts with init()
func ParseFlags() error {
	flag.StringVar(&Conf.ShortLinkCharset, "c", DefaultCharset, "chars to use in id generator")
	flag.StringVar(&Conf.RunAddr, "a", ":8080", "address and port to run server")
	flag.StringVar(&Conf.BaseResultAddr, "b", "http://localhost:8080", "base url for short link")
	flag.IntVar(&Conf.ShortLinkLength, "l", 6, "short link id length, max length 12")
	flag.StringVar(&Conf.LogLevel, "ll", "info", "log level")
	flag.StringVar(&Conf.FileStoragePath, "f", "", "file storage path")
	// sslmode=disable for local db in docker
	flag.StringVar(&Conf.DBDSN, "d", "", "database dsn string. format: postgres://user:pass@localhost:5432/db?sslmode=disable")
	flag.IntVar(&Conf.TokenExpMinutes, "te", 5, "token expire time in minutes")
	flag.StringVar(&Conf.TokenSecret, "ts", "", "token secret")
	flag.Parse()

	err := env.Parse(&Conf)
	if err != nil {
		return err
	}
	if Conf.ShortLinkLength > 12 {
		return fmt.Errorf("short link id length should not be more than 12: %v", Conf.ShortLinkLength)
	}

	Conf.DetectStorageType()

	return nil
}
