package config

import (
	"flag"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func withTestFlagsAndEnv(t *testing.T, args []string, env map[string]string) {
	t.Helper()

	originalCommandLine := flag.CommandLine
	originalArgs := os.Args
	originalConf := Conf

	flag.CommandLine = flag.NewFlagSet(args[0], flag.ContinueOnError)
	os.Args = args
	Conf = Config{}

	for key, value := range env {
		t.Setenv(key, value)
	}

	t.Cleanup(func() {
		flag.CommandLine = originalCommandLine
		os.Args = originalArgs
		Conf = originalConf
	})
}

func TestParseFlags(t *testing.T) {
	t.Run("uses default values", func(t *testing.T) {
		withTestFlagsAndEnv(t, []string{"shortener"}, nil)

		require.NoError(t, ParseFlags())

		assert.Equal(t, DefaultCharset, Conf.ShortLinkCharset)
		assert.Equal(t, ":8080", Conf.RunAddr)
		assert.Equal(t, "http://localhost:8080", Conf.BaseResultAddr)
		assert.Equal(t, 6, Conf.ShortLinkLength)
		assert.Equal(t, "info", Conf.LogLevel)
		assert.Empty(t, Conf.FileStoragePath)
		assert.Empty(t, Conf.DBDSN)
		assert.Equal(t, InMemory, Conf.StorageType)
	})

	t.Run("uses flag values", func(t *testing.T) {
		withTestFlagsAndEnv(t, []string{
			"shortener",
			"-c", "abc123",
			"-a", ":9090",
			"-b", "https://short.example.com",
			"-l", "10",
			"-ll", "debug",
			"-f", "/tmp/flag-storage.json",
			"-d", "postgres://flag-user:flag-pass@localhost:5432/flag-db",
		}, nil)

		require.NoError(t, ParseFlags())

		assert.Equal(t, "abc123", Conf.ShortLinkCharset)
		assert.Equal(t, ":9090", Conf.RunAddr)
		assert.Equal(t, "https://short.example.com", Conf.BaseResultAddr)
		assert.Equal(t, 10, Conf.ShortLinkLength)
		assert.Equal(t, "debug", Conf.LogLevel)
		assert.Equal(t, "/tmp/flag-storage.json", Conf.FileStoragePath)
		assert.Equal(t, "postgres://flag-user:flag-pass@localhost:5432/flag-db", Conf.DBDSN)
		assert.Equal(t, Database, Conf.StorageType)
	})

	t.Run("uses file storage when database DSN is empty", func(t *testing.T) {
		withTestFlagsAndEnv(t, []string{
			"shortener",
			"-f", "/tmp/storage.json",
		}, nil)

		require.NoError(t, ParseFlags())

		assert.Equal(t, "/tmp/storage.json", Conf.FileStoragePath)
		assert.Empty(t, Conf.DBDSN)
		assert.Equal(t, File, Conf.StorageType)
	})

	t.Run("environment variables override flag values", func(t *testing.T) {
		withTestFlagsAndEnv(t, []string{
			"shortener",
			"-c", "abcde",
			"-a", ":9090",
			"-b", "https://flag.example.com",
			"-l", "10",
			"-ll", "debug",
			"-f", "/tmp/flag-storage.json",
			"-d", "postgres://flag-user:flag-pass@localhost:5432/flag-db",
		}, map[string]string{
			"SH_CHARSET":        "xyz",
			"SERVER_ADDRESS":    ":7070",
			"BASE_URL":          "https://env.example.com",
			"SH_LENGTH":         "12",
			"LOG_LEVEL":         "warn",
			"FILE_STORAGE_PATH": "/tmp/env-storage.json",
			"DATABASE_DSN":      "postgres://env-user:env-pass@localhost:5432/env-db",
		})

		require.NoError(t, ParseFlags())

		assert.Equal(t, "xyz", Conf.ShortLinkCharset)
		assert.Equal(t, ":7070", Conf.RunAddr)
		assert.Equal(t, "https://env.example.com", Conf.BaseResultAddr)
		assert.Equal(t, 12, Conf.ShortLinkLength)
		assert.Equal(t, "warn", Conf.LogLevel)
		assert.Equal(t, "/tmp/env-storage.json", Conf.FileStoragePath)
		assert.Equal(t, "postgres://env-user:env-pass@localhost:5432/env-db", Conf.DBDSN)
		assert.Equal(t, Database, Conf.StorageType)
	})

	t.Run("returns error when short link length exceeds maximum", func(t *testing.T) {
		withTestFlagsAndEnv(t, []string{
			"shortener",
			"-l", "13",
		}, nil)

		err := ParseFlags()

		assert.EqualError(t, err, "short link id length should not be more than 12: 13")
	})

	t.Run("returns error for invalid environment value", func(t *testing.T) {
		withTestFlagsAndEnv(t, []string{"shortener"}, map[string]string{
			"SH_LENGTH": "not-a-number",
		})

		err := ParseFlags()

		require.Error(t, err)
		assert.ErrorContains(t, err, "ShortLinkLength")
	})
}
