package logger

import "go.uber.org/zap"

func New(env string) (*zap.Logger, error) {
	if env == "production" {
		return zap.NewProduction()
	}
	return zap.NewDevelopment()
}

func Error(err error) zap.Field {
	return zap.Error(err)
}
