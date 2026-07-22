package config

import (
	"flag"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
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
		dir, err := os.Getwd()
		assert.NoError(t, err)

		ParseFlags()

		assert.Equal(t, DefaultCharset, Conf.ShortLinkCharset)
		assert.Equal(t, ":8080", Conf.RunAddr)
		assert.Equal(t, "http://localhost:8080", Conf.BaseResultAddr)
		assert.Equal(t, 6, Conf.ShortLinkLength)
		assert.Equal(t, "info", Conf.LogLevel)
		assert.Equal(t, dir+"/storage.json", Conf.FileStoragePath)
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
		}, nil)

		ParseFlags()

		assert.Equal(t, "abc123", Conf.ShortLinkCharset)
		assert.Equal(t, ":9090", Conf.RunAddr)
		assert.Equal(t, "https://short.example.com", Conf.BaseResultAddr)
		assert.Equal(t, 10, Conf.ShortLinkLength)
		assert.Equal(t, "debug", Conf.LogLevel)
		assert.Equal(t, "/tmp/flag-storage.json", Conf.FileStoragePath)
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
		}, map[string]string{
			"SH_CHARSET":         "xyz",
			"SERVER_ADDRESS":     ":7070",
			"BASE_URL":           "https://env.example.com",
			"SH_LENGTH":          "12",
			"LOG_LEVEL":          "warn",
			"FILE_STORAGE_PATH": "/tmp/env-storage.json",
		})

		ParseFlags()

		assert.Equal(t, "xyz", Conf.ShortLinkCharset)
		assert.Equal(t, ":7070", Conf.RunAddr)
		assert.Equal(t, "https://env.example.com", Conf.BaseResultAddr)
		assert.Equal(t, 12, Conf.ShortLinkLength)
		assert.Equal(t, "warn", Conf.LogLevel)
		assert.Equal(t, "/tmp/env-storage.json", Conf.FileStoragePath)
	})

}
