package logger

import (
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
		require.NoError(t, Log.Sync())
	})

	t.Run("returns error for invalid level", func(t *testing.T) {
		err := InitLogger("not-a-log-level")
		require.Error(t, err)
	})
}