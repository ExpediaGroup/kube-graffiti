package log

import (
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func ComponentLogger(component, funcname string) zerolog.Logger {
	logger := log.Logger.With().Str("component", component).Logger()
	if zerolog.GlobalLevel() == zerolog.DebugLevel {
		logger = logger.With().Str("func", funcname).Logger()
	}
	return logger
}
