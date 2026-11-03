package config

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

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

// Config holds application settings loaded from JSON, flags and environment variables.
type Config struct {
	// RunAddr is the HTTP server listen address.
	RunAddr string `env:"SERVER_ADDRESS" json:"server_address"`
	// EnableHTTPS enables TLS for the application server.
	EnableHTTPS bool `env:"ENABLE_HTTPS" json:"enable_https"`
	// TLSCertFile is the path to the PEM-encoded server certificate chain.
	TLSCertFile string `env:"TLS_CERT_FILE" json:"tls_cert_file"`
	// TLSKeyFile is the path to the PEM-encoded server private key.
	TLSKeyFile string `env:"TLS_KEY_FILE" json:"tls_key_file"`
	// BaseResultAddr is the URL prefix used in short link responses.
	BaseResultAddr string `env:"BASE_URL" json:"base_url"`
	// ShortLinkCharset contains the bytes available for ID generation.
	ShortLinkCharset string `env:"SH_CHARSET" json:"sh_charset"`
	// LogLevel is the logging level understood by zap.
	LogLevel string `env:"LOG_LEVEL" json:"log_level"`
	// FileStoragePath is the path to the JSON link storage file.
	FileStoragePath string `env:"FILE_STORAGE_PATH" json:"file_storage_path"`
	// DBDSN is the PostgreSQL connection string.
	DBDSN string `env:"DATABASE_DSN" json:"database_dsn"`

	// TokenSecret is the secret used to sign and verify authentication tokens.
	TokenSecret string `env:"TOKEN_SECRET" json:"token_secret"`
	// AuditFile is the path to the audit log file.
	AuditFile string `env:"AUDIT_FILE" json:"audit_file"`
	// AuditURL is the HTTP endpoint receiving audit events.
	AuditURL string `env:"AUDIT_URL" json:"audit_url"`

	// ShortLinkLength is the generated ID length in bytes.
	ShortLinkLength int `env:"SH_LENGTH" json:"sh_length"`
	// TokenExpMinutes is the authentication token lifetime in minutes.
	TokenExpMinutes int `env:"TOKEN_EXP_MINUTES" json:"token_exp_minutes"`
	// StorageType is the backend selected by DetectStorageType.
	StorageType StorageType `json:"-"`

	// UseProfiler is a flag to enable profiler server
	UseProfiler bool `env:"USE_PROFILER" json:"use_profiler"`
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

// ParseFlags loads settings with precedence: environment > flags > JSON > defaults.
// The JSON path is selected by CONFIG, then -c/-config; no file is required by default.
// It returns an error for invalid flags, files, environment values or ID lengths over 12.
// It must be called only once per flag set to avoid duplicate registrations.
func ParseFlags() error {
	var configPath string
	flag.StringVar(&configPath, "c", "", "path to JSON configuration file")
	flag.StringVar(&configPath, "config", "", "path to JSON configuration file")
	flag.StringVar(&Conf.ShortLinkCharset, "charset", DefaultCharset, "chars to use in id generator")
	flag.StringVar(&Conf.RunAddr, "a", ":8080", "address and port to run server")
	flag.BoolVar(&Conf.EnableHTTPS, "s", false, "enable HTTPS")
	flag.StringVar(&Conf.TLSCertFile, "tls-cert", "cert.pem", "path to TLS certificate PEM file")
	flag.StringVar(&Conf.TLSKeyFile, "tls-key", "key.pem", "path to TLS private key PEM file")
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
	flag.BoolVar(&Conf.UseProfiler, "p", false, "enable profiler")
	defaults := Conf
	if err := flag.CommandLine.Parse(os.Args[1:]); err != nil {
		return err
	}
	if path := os.Getenv("CONFIG"); path != "" {
		configPath = path
	}
	if configPath != "" {
		data, err := os.ReadFile(configPath)
		if err != nil {
			return fmt.Errorf("read config file %q: %w", configPath, err)
		}
		if err := json.Unmarshal(data, &defaults); err != nil {
			return fmt.Errorf("parse config file %q: %w", configPath, err)
		}

		// Save only explicitly supplied flags, including false, zero and empty values.
		values := make(map[string]string)
		flag.Visit(func(f *flag.Flag) {
			values[f.Name] = f.Value.String()
		})
		Conf = defaults
		for name, value := range values {
			if err := flag.Set(name, value); err != nil {
				return fmt.Errorf("restore flag %q: %w", name, err)
			}
		}
	}

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
