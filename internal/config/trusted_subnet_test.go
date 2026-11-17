package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTrustedSubnetConfig(t *testing.T) {
	path := writeConfigFile(t, `{"trusted_subnet":"10.0.0.0/8"}`)
	for _, tt := range []struct {
		name string
		args []string
		env  map[string]string
		want string
	}{
		{"default", nil, nil, ""},
		{"JSON", []string{"-c", path}, nil, "10.0.0.0/8"},
		{"flag", []string{"-t", "192.168.0.0/16"}, nil, "192.168.0.0/16"},
		{"flag overrides JSON", []string{"-c", path, "-t", "::1/128"}, nil, "::1/128"},
		{"empty flag overrides JSON", []string{"-c", path, "-t="}, nil, ""},
		{"environment", nil, map[string]string{"TRUSTED_SUBNET": "::1/128"}, "::1/128"},
		{"environment overrides flag and JSON", []string{"-c", path, "-t", "192.168.0.0/16"}, map[string]string{"TRUSTED_SUBNET": "::1/128"}, "::1/128"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			withTestFlagsAndEnv(t, append([]string{"shortener"}, tt.args...), tt.env)
			require.NoError(t, ParseFlags())
			require.Equal(t, tt.want, Conf.TrustedSubnet)
		})
	}
}
