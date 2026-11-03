package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeConfigFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	require.NoError(t, os.WriteFile(path, []byte(content), 0600))
	return path
}

func TestParseFlagsJSON(t *testing.T) {
	path := writeConfigFile(t, `{
		"server_address": "localhost:9090",
		"base_url": "https://short.example",
		"file_storage_path": "links.json",
		"database_dsn": "postgres://localhost/shortener",
		"enable_https": true,
		"tls_cert_file": "server.pem",
		"tls_key_file": "private.pem",
		"sh_charset": "abc123",
		"sh_length": 10,
		"log_level": "debug",
		"token_secret": "secret",
		"token_exp_minutes": 30,
		"audit_file": "audit.json",
		"audit_url": "https://audit.example",
		"use_profiler": true
	}`)
	expected := Config{
		RunAddr: "localhost:9090", BaseResultAddr: "https://short.example",
		FileStoragePath: "links.json", DBDSN: "postgres://localhost/shortener",
		EnableHTTPS: true, TLSCertFile: "server.pem", TLSKeyFile: "private.pem",
		ShortLinkCharset: "abc123", ShortLinkLength: 10, LogLevel: "debug",
		TokenSecret: "secret", TokenExpMinutes: 30,
		AuditFile: "audit.json", AuditURL: "https://audit.example",
		UseProfiler: true, StorageType: Database,
	}
	for _, args := range [][]string{
		{"shortener", "-c", path},
		{"shortener", "-config", path},
		{"shortener", "-c=" + path},
		{"shortener", "-config=" + path},
	} {
		t.Run(args[1], func(t *testing.T) {
			withTestFlagsAndEnv(t, args, nil)
			require.NoError(t, ParseFlags())
			assert.Equal(t, expected, Conf)
		})
	}
	t.Run("CONFIG overrides path flags", func(t *testing.T) {
		withTestFlagsAndEnv(t, []string{"shortener", "-c", "missing.json", "-config", "also-missing.json"}, map[string]string{"CONFIG": path})
		require.NoError(t, ParseFlags())
		assert.Equal(t, expected, Conf)
	})
	t.Run("CONFIG alone", func(t *testing.T) {
		withTestFlagsAndEnv(t, []string{"shortener"}, map[string]string{"CONFIG": path})
		require.NoError(t, ParseFlags())
		assert.Equal(t, expected, Conf)
	})
	t.Run("all explicit flags override file including zero values", func(t *testing.T) {
		withTestFlagsAndEnv(t, []string{
			"shortener", "-a", ":8080", "-config", path,
			"-b", "http://localhost:8080", "-f=", "-d=", "-s=false",
			"-tls-cert", "cert.pem", "-tls-key", "key.pem",
			"-charset", DefaultCharset, "-l", "6", "-ll", "info",
			"-ts=", "-te", "0", "-audit-file=", "-audit-url=", "-p=false",
		}, nil)
		require.NoError(t, ParseFlags())
		assert.Equal(t, Config{
			RunAddr: ":8080", BaseResultAddr: "http://localhost:8080",
			TLSCertFile: "cert.pem", TLSKeyFile: "key.pem",
			ShortLinkCharset: DefaultCharset, ShortLinkLength: 6, LogLevel: "info",
		}, Conf)
	})
	t.Run("environment overrides file and flags", func(t *testing.T) {
		withTestFlagsAndEnv(t, []string{"shortener", "-c", path, "-a", ":8081", "-s"}, map[string]string{
			"SERVER_ADDRESS": ":8082", "BASE_URL": "http://env.example",
			"FILE_STORAGE_PATH": "env.json", "DATABASE_DSN": "env-dsn",
			"ENABLE_HTTPS": "false", "TLS_CERT_FILE": "env-cert", "TLS_KEY_FILE": "env-key",
			"SH_CHARSET": "xyz", "SH_LENGTH": "7", "LOG_LEVEL": "warn",
			"TOKEN_SECRET": "env-secret", "TOKEN_EXP_MINUTES": "0",
			"AUDIT_FILE": "env-audit", "AUDIT_URL": "http://env-audit", "USE_PROFILER": "false",
		})
		require.NoError(t, ParseFlags())
		assert.Equal(t, Config{
			RunAddr: ":8082", BaseResultAddr: "http://env.example",
			FileStoragePath: "env.json", DBDSN: "env-dsn", StorageType: Database,
			TLSCertFile: "env-cert", TLSKeyFile: "env-key",
			ShortLinkCharset: "xyz", ShortLinkLength: 7, LogLevel: "warn",
			TokenSecret: "env-secret", TokenExpMinutes: 0,
			AuditFile: "env-audit", AuditURL: "http://env-audit",
		}, Conf)
	})
}

func TestParseFlagsJSONDefaultsAndErrors(t *testing.T) {
	for _, content := range []string{`{}`, `{"file_storage_path":"links.json"}`} {
		t.Run(content, func(t *testing.T) {
			path := writeConfigFile(t, content)
			withTestFlagsAndEnv(t, []string{"shortener", "-c", path}, nil)
			require.NoError(t, ParseFlags())
			assert.Equal(t, ":8080", Conf.RunAddr)
			assert.Equal(t, "http://localhost:8080", Conf.BaseResultAddr)
			assert.Equal(t, DefaultCharset, Conf.ShortLinkCharset)
			assert.Equal(t, 6, Conf.ShortLinkLength)
			assert.Equal(t, 5, Conf.TokenExpMinutes)
			assert.Equal(t, "info", Conf.LogLevel)
			assert.Equal(t, "cert.pem", Conf.TLSCertFile)
			assert.Equal(t, "key.pem", Conf.TLSKeyFile)
			assert.False(t, Conf.EnableHTTPS)
			assert.False(t, Conf.UseProfiler)
			if content == `{}` {
				assert.Equal(t, InMemory, Conf.StorageType)
			} else {
				assert.Equal(t, File, Conf.StorageType)
			}
		})
	}
	for _, content := range []string{"", `{`, `[]`, `{"enable_https":"true"}`, `{"sh_length":1.5}`, `{} {}`} {
		t.Run("invalid JSON "+content, func(t *testing.T) {
			path := writeConfigFile(t, content)
			withTestFlagsAndEnv(t, []string{"shortener", "-c", path}, nil)
			err := ParseFlags()
			require.ErrorContains(t, err, "parse config file")
		})
	}
	t.Run("missing file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "missing.json")
		withTestFlagsAndEnv(t, []string{"shortener", "-c", path}, nil)
		require.ErrorIs(t, ParseFlags(), os.ErrNotExist)
	})
	t.Run("validates merged length", func(t *testing.T) {
		path := writeConfigFile(t, `{"sh_length":13}`)
		withTestFlagsAndEnv(t, []string{"shortener", "-c", path}, nil)
		require.ErrorContains(t, ParseFlags(), "should not be more than 12")
	})
	t.Run("flag fixes file length", func(t *testing.T) {
		path := writeConfigFile(t, `{"sh_length":13}`)
		withTestFlagsAndEnv(t, []string{"shortener", "-c", path, "-l", "8"}, nil)
		require.NoError(t, ParseFlags())
		assert.Equal(t, 8, Conf.ShortLinkLength)
	})
	t.Run("invalid flag", func(t *testing.T) {
		withTestFlagsAndEnv(t, []string{"shortener", "-s=invalid"}, nil)
		require.Error(t, ParseFlags())
	})
}
