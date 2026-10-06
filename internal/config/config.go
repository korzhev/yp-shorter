package config

import (
	"flag"
	"fmt"

	"github.com/caarlos0/env/v6"
)

// DefaultCharset contains the default characters used to generate short link IDs.
const DefaultCharset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// StorageType identifies the storage backend for short links.
type StorageType int

const (
	// InMemory stores links only in memory.
	InMemory StorageType = iota
	// File stores links in memory and persists changes to a JSON file.
	File
	// Database stores links in PostgreSQL.
	Database
)

// Config holds application settings loaded from flags and environment variables.
type Config struct {
	// ShortLinkLength is the generated ID length in bytes.
	ShortLinkLength int `env:"SH_LENGTH"`
	// RunAddr is the HTTP server listen address.
	RunAddr string `env:"SERVER_ADDRESS"`
	// BaseResultAddr is the URL prefix used in short link responses.
	BaseResultAddr string `env:"BASE_URL"`
	// ShortLinkCharset contains the bytes available for ID generation.
	ShortLinkCharset string `env:"SH_CHARSET"`
	// LogLevel is the logging level understood by zap.
	LogLevel string `env:"LOG_LEVEL"`
	// FileStoragePath is the path to the JSON link storage file.
	FileStoragePath string `env:"FILE_STORAGE_PATH"`
	// DBDSN is the PostgreSQL connection string.
	DBDSN string `env:"DATABASE_DSN"`
	// TokenExpMinutes is the authentication token lifetime in minutes.
	TokenExpMinutes int `env:"TOKEN_EXP_MINUTES"`
	// TokenSecret is the secret used to sign and verify authentication tokens.
	TokenSecret string `env:"TOKEN_SECRET"`
	// AuditFile is the path to the audit log file.
	AuditFile string `env:"AUDIT_FILE"`
	// AuditURL is the HTTP endpoint receiving audit events.
	AuditURL string `env:"AUDIT_URL"`
	// StorageType is the backend selected by DetectStorageType.
	StorageType StorageType
}

// DetectStorageType updates and returns the backend, preferring DBDSN over
// FileStoragePath and falling back to in-memory storage when both are empty.
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

// Conf is the shared application configuration populated by ParseFlags.
var Conf Config

// ParseFlags registers and parses command-line flags, then applies environment
// overrides to Conf and selects the storage backend. It returns an error for
// invalid environment values or an ID length greater than 12.
// It must be called only once per flag set to avoid duplicate registrations.
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
	flag.StringVar(&Conf.AuditFile, "audit-file", "", "path to audit file")
	flag.StringVar(&Conf.AuditURL, "audit-url", "", "audit URL")
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
