package logger

import (
	"errors"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zapcore"
)

func TestInitLogger(t *testing.T) {
	t.Run("initializes logger with requested level", func(t *testing.T) {
		require.NoError(t, InitLogger("debug"))
		require.NotNil(t, Log)
		assert.True(t, Log.Desugar().Core().Enabled(zapcore.DebugLevel))

		// Sync calls fsync for stderr. Terminals and /dev/stderr may not support
		// fsync and return EINVAL even though logging itself works correctly.
		if err := Log.Sync(); err != nil {
			require.True(t, errors.Is(err, syscall.EINVAL), "unexpected sync error: %v", err)
		}
	})

	t.Run("returns error for invalid level", func(t *testing.T) {
		err := InitLogger("not-a-log-level")
		require.Error(t, err)
	})
}
