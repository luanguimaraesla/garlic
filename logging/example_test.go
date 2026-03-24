package logging_test

import (
	"go.uber.org/zap"

	"github.com/luanguimaraesla/garlic/logging"
)

func ExampleInit() {
	logging.Init(&logging.Config{
		Level:    "info",
		Encoding: "json",
	})

	logger := logging.Global()
	logger.Info("application started", zap.String("version", "1.0.0"))
}
