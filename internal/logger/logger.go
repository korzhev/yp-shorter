package logger

import (
	"go.uber.org/zap"
)

// Log is the shared application logger. Call InitLogger before using it.
var Log *zap.SugaredLogger

// InitLogger initializes Log with zap's production configuration and level.
// It returns an error if the level is invalid or the logger cannot be built.
func InitLogger(level string) error {
	lvl, err := zap.ParseAtomicLevel(level)
	if err != nil {
		return err
	}
	cfg := zap.NewProductionConfig()
	cfg.Level = lvl
	zl, err := cfg.Build()
	if err != nil {
		return err
	}
	Log = zl.Sugar()
	return nil
}
