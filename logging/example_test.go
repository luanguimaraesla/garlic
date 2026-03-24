package logging_test

import (
	"github.com/luanguimaraesla/garlic/logging"
	"go.uber.org/zap"
)

func ExampleInit() {
	logging.Init(&logging.Config{
		Level:    "info",
		Encoding: "json",
	})

	logger := logging.Global()
	logger.Info("application started", zap.String("version", "1.0.0"))
}
