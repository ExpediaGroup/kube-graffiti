/*
Copyright (C) 2018 Expedia Group.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package log

import (
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var (
	// LogLevels defines a map of valid log level strings to their corresponding zerolog types.
	LogLevels = map[string]zerolog.Level{
		"panic": zerolog.DebugLevel,
		"fatal": zerolog.FatalLevel,
		"error": zerolog.ErrorLevel,
		"warn":  zerolog.WarnLevel,
		"info":  zerolog.InfoLevel,
		"debug": zerolog.DebugLevel,
	}
)

// InitLogger sets up our logger with default level and output to console
func InitLogger(level string) {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	// set level width if PR https://github.com/rs/zerolog/pull/87 is accepted
	// zerolog.LevelWidth = 5
	zerolog.SetGlobalLevel(LogLevels[level])
}

// ChangeLogLevel allows the changing of the global log level
func ChangeLogLevel(level string) {
	// set level width if PR https://github.com/rs/zerolog/pull/87 is accepted
	// zerolog.LevelWidth = 5
	zerolog.SetGlobalLevel(LogLevels[level])
}

func ComponentLogger(component, funcname string) zerolog.Logger {
	logger := log.Logger.With().Str("component", component).Logger()
	if zerolog.GlobalLevel() == zerolog.DebugLevel {
		logger = logger.With().Str("func", funcname).Logger()
	}
	return logger
}
