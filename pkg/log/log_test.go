package log_test

import (
	"context"
	"testing"

	"github.com/Nordix/xcluster-cni/pkg/log"
	"github.com/go-logr/logr"
	"go.uber.org/zap"
)

func zapLogger(level string) *zap.Logger {
	z, _ := log.ZapLogger("stderr", level)
	return z
}

func TestLogger(t *testing.T) {

	ctx := log.NewContext(context.Background(), zapLogger("trace"))
	logger := logr.FromContextOrDiscard(ctx)
	logger.Info("Hello, world")
	logger.V(2).Info("You should see this")
	ctx = log.NewContext(ctx, zapLogger("")) // no-op
	logger = logr.FromContextOrDiscard(ctx)
	logger.V(2).Info("You should STILL see this")

	ctx = log.NewContext(context.Background(), zapLogger(""))
	logger = logr.FromContextOrDiscard(ctx)
	logger.V(2).Info("You should NOT see this")
	logger.V(1).Info("You should NOT see this")
	logger.Info("Info level log")

	ctx = log.NewContext(context.Background(), zapLogger("10"))
	logger = logr.FromContextOrDiscard(ctx)
	logger.V(2).Info("You should see this (2)")
	logger.V(10).Info("You should see this (10)")
	logger.V(11).Info("You should NOT see this (11)")
}
