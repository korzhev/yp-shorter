package config

import (
	"flag"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var configEnvKeys = []string{
	"SH_LENGTH",
	"SERVER_ADDRESS",
	"BASE_URL",
	"SH_CHARSET",
	"LOG_LEVEL",
	"FILE_STORAGE_PATH",
	"DATABASE_DSN",
	"TOKEN_EXP_MINUTES",
	"TOKEN_SECRET",
	"AUDIT_FILE",
	"AUDIT_URL",
	"USE_PROFILER",
	"ENABLE_HTTPS",
	"TLS_CERT_FILE",
	"TLS_KEY_FILE",
}

func withTestFlagsAndEnv(t *testing.T, args []string, env map[string]string) {
	t.Helper()

	originalCommandLine := flag.CommandLine
	originalArgs := os.Args
	originalConf := Conf
	originalEnv := make(map[string]string, len(configEnvKeys))
	setEnvKeys := make(map[string]bool, len(configEnvKeys))

	for _, key := range configEnvKeys {
		originalEnv[key], setEnvKeys[key] = os.LookupEnv(key)
	}

	t.Cleanup(func() {
		flag.CommandLine = originalCommandLine
		os.Args = originalArgs
		Conf = originalConf

		for _, key := range configEnvKeys {
			if setEnvKeys[key] {
				require.NoError(t, os.Setenv(key, originalEnv[key]))
				continue
			}
			require.NoError(t, os.Unsetenv(key))
		}
	})

	flag.CommandLine = flag.NewFlagSet(args[0], flag.ContinueOnError)
	os.Args = args
	Conf = Config{}

	for _, key := range configEnvKeys {
		require.NoError(t, os.Unsetenv(key))
	}

	for key, value := range env {
		require.NoError(t, os.Setenv(key, value))
	}
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
		assert.Equal(t, 5, Conf.TokenExpMinutes)
		assert.Empty(t, Conf.TokenSecret)
		assert.Empty(t, Conf.AuditFile)
		assert.Empty(t, Conf.AuditURL)
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
			"-te", "30",
			"-ts", "flag-secret",
			"-audit-file", "/tmp/flag-audit.json",
			"-audit-url", "https://flag.example.com/audit",
		}, nil)

		require.NoError(t, ParseFlags())

		assert.Equal(t, "abc123", Conf.ShortLinkCharset)
		assert.Equal(t, ":9090", Conf.RunAddr)
		assert.Equal(t, "https://short.example.com", Conf.BaseResultAddr)
		assert.Equal(t, 10, Conf.ShortLinkLength)
		assert.Equal(t, "debug", Conf.LogLevel)
		assert.Equal(t, "/tmp/flag-storage.json", Conf.FileStoragePath)
		assert.Equal(t, "postgres://flag-user:flag-pass@localhost:5432/flag-db", Conf.DBDSN)
		assert.Equal(t, 30, Conf.TokenExpMinutes)
		assert.Equal(t, "flag-secret", Conf.TokenSecret)
		assert.Equal(t, "/tmp/flag-audit.json", Conf.AuditFile)
		assert.Equal(t, "https://flag.example.com/audit", Conf.AuditURL)
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
			"-te", "30",
			"-ts", "flag-secret",
			"-audit-file", "/tmp/flag-audit.json",
			"-audit-url", "https://flag.example.com/audit",
		}, map[string]string{
			"SH_CHARSET":        "xyz",
			"SERVER_ADDRESS":    ":7070",
			"BASE_URL":          "https://env.example.com",
			"SH_LENGTH":         "12",
			"LOG_LEVEL":         "warn",
			"FILE_STORAGE_PATH": "/tmp/env-storage.json",
			"DATABASE_DSN":      "postgres://env-user:env-pass@localhost:5432/env-db",
			"TOKEN_EXP_MINUTES": "60",
			"TOKEN_SECRET":      "env-secret",
			"AUDIT_FILE":        "/tmp/env-audit.json",
			"AUDIT_URL":         "https://env.example.com/audit",
		})

		require.NoError(t, ParseFlags())

		assert.Equal(t, "xyz", Conf.ShortLinkCharset)
		assert.Equal(t, ":7070", Conf.RunAddr)
		assert.Equal(t, "https://env.example.com", Conf.BaseResultAddr)
		assert.Equal(t, 12, Conf.ShortLinkLength)
		assert.Equal(t, "warn", Conf.LogLevel)
		assert.Equal(t, "/tmp/env-storage.json", Conf.FileStoragePath)
		assert.Equal(t, "postgres://env-user:env-pass@localhost:5432/env-db", Conf.DBDSN)
		assert.Equal(t, 60, Conf.TokenExpMinutes)
		assert.Equal(t, "env-secret", Conf.TokenSecret)
		assert.Equal(t, "/tmp/env-audit.json", Conf.AuditFile)
		assert.Equal(t, "https://env.example.com/audit", Conf.AuditURL)
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

	t.Run("returns error for invalid token expiration", func(t *testing.T) {
		withTestFlagsAndEnv(t, []string{"shortener"}, map[string]string{
			"TOKEN_EXP_MINUTES": "not-a-number",
		})

		err := ParseFlags()

		require.Error(t, err)
		assert.ErrorContains(t, err, "TokenExpMinutes")
	})
}

func TestParseFlagsHTTPS(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		env       map[string]string
		want      bool
		wantError bool
	}{
		{name: "disabled by default"},
		{name: "bare flag", args: []string{"-s"}, want: true},
		{name: "explicit true", args: []string{"-s=true"}, want: true},
		{name: "explicit false", args: []string{"-s=false"}},
		{name: "environment true", env: map[string]string{"ENABLE_HTTPS": "true"}, want: true},
		{name: "environment false", env: map[string]string{"ENABLE_HTTPS": "false"}},
		{name: "environment disables flag", args: []string{"-s"}, env: map[string]string{"ENABLE_HTTPS": "false"}},
		{name: "environment enables flag", args: []string{"-s=false"}, env: map[string]string{"ENABLE_HTTPS": "true"}, want: true},
		{name: "invalid environment", env: map[string]string{"ENABLE_HTTPS": "invalid"}, wantError: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withTestFlagsAndEnv(t, append([]string{"shortener"}, tt.args...), tt.env)
			err := ParseFlags()
			if tt.wantError {
				require.ErrorContains(t, err, "EnableHTTPS")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, Conf.EnableHTTPS)
			assert.Equal(t, "cert.pem", Conf.TLSCertFile)
			assert.Equal(t, "key.pem", Conf.TLSKeyFile)
		})
	}
	t.Run("certificate flags", func(t *testing.T) {
		withTestFlagsAndEnv(t, []string{"shortener", "-tls-cert", "flag-cert.pem", "-tls-key", "flag-key.pem"}, nil)
		require.NoError(t, ParseFlags())
		assert.Equal(t, "flag-cert.pem", Conf.TLSCertFile)
		assert.Equal(t, "flag-key.pem", Conf.TLSKeyFile)
	})
	t.Run("certificate environment overrides flags", func(t *testing.T) {
		withTestFlagsAndEnv(t, []string{"shortener", "-tls-cert", "flag-cert.pem", "-tls-key", "flag-key.pem"}, map[string]string{
			"TLS_CERT_FILE": "env-cert.pem", "TLS_KEY_FILE": "env-key.pem",
		})
		require.NoError(t, ParseFlags())
		assert.Equal(t, "env-cert.pem", Conf.TLSCertFile)
		assert.Equal(t, "env-key.pem", Conf.TLSKeyFile)
	})
}

func TestParseFlagsUseProfiler(t *testing.T) {
	t.Run("disabled by default", func(t *testing.T) {
		withTestFlagsAndEnv(t, []string{"shortener"}, nil)

		require.NoError(t, ParseFlags())
		assert.False(t, Conf.UseProfiler)
	})

	t.Run("enabled by bare flag", func(t *testing.T) {
		withTestFlagsAndEnv(t, []string{"shortener", "-p"}, nil)

		require.NoError(t, ParseFlags())
		assert.True(t, Conf.UseProfiler)
	})

	t.Run("enabled by explicit flag value", func(t *testing.T) {
		withTestFlagsAndEnv(t, []string{"shortener", "-p=true"}, nil)

		require.NoError(t, ParseFlags())
		assert.True(t, Conf.UseProfiler)
	})

	t.Run("disabled by explicit flag value", func(t *testing.T) {
		withTestFlagsAndEnv(t, []string{"shortener", "-p=false"}, nil)

		require.NoError(t, ParseFlags())
		assert.False(t, Conf.UseProfiler)
	})

	t.Run("enabled by environment variable", func(t *testing.T) {
		withTestFlagsAndEnv(t, []string{"shortener"}, map[string]string{
			"USE_PROFILER": "true",
		})

		require.NoError(t, ParseFlags())
		assert.True(t, Conf.UseProfiler)
	})

	t.Run("disabled by environment variable", func(t *testing.T) {
		withTestFlagsAndEnv(t, []string{"shortener"}, map[string]string{
			"USE_PROFILER": "false",
		})

		require.NoError(t, ParseFlags())
		assert.False(t, Conf.UseProfiler)
	})

	t.Run("environment disables profiler enabled by flag", func(t *testing.T) {
		withTestFlagsAndEnv(t, []string{"shortener", "-p"}, map[string]string{
			"USE_PROFILER": "false",
		})

		require.NoError(t, ParseFlags())
		assert.False(t, Conf.UseProfiler)
	})

	t.Run("environment enables profiler disabled by flag", func(t *testing.T) {
		withTestFlagsAndEnv(t, []string{"shortener", "-p=false"}, map[string]string{
			"USE_PROFILER": "true",
		})

		require.NoError(t, ParseFlags())
		assert.True(t, Conf.UseProfiler)
	})

	t.Run("returns error for invalid environment value", func(t *testing.T) {
		withTestFlagsAndEnv(t, []string{"shortener"}, map[string]string{
			"USE_PROFILER": "not-a-bool",
		})

		err := ParseFlags()

		require.Error(t, err)
		assert.ErrorContains(t, err, "UseProfiler")
	})
}

func TestConfig_DetectStorageType(t *testing.T) {
	tests := []struct {
		name     string
		config   Config
		expected StorageType
	}{
		{
			name:     "in-memory storage by default",
			config:   Config{},
			expected: InMemory,
		},
		{
			name: "file storage",
			config: Config{
				FileStoragePath: "/tmp/storage.json",
			},
			expected: File,
		},
		{
			name: "database storage",
			config: Config{
				DBDSN: "postgres://user:pass@localhost:5432/db",
			},
			expected: Database,
		},
		{
			name: "database takes precedence over file storage",
			config: Config{
				FileStoragePath: "/tmp/storage.json",
				DBDSN:           "postgres://user:pass@localhost:5432/db",
			},
			expected: Database,
		},
		{
			name: "resets previously detected storage type",
			config: Config{
				StorageType: Database,
			},
			expected: InMemory,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := tt.config.DetectStorageType()

			assert.Equal(t, tt.expected, actual)
			assert.Equal(t, tt.expected, tt.config.StorageType)
		})
	}
}
