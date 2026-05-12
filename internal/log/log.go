package log

import (
	"future_was/internal/app"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gopkg.in/natefinch/lumberjack.v2"
)

// 로그 메타데이터 키
const (
	logProject = "project"
	logHost    = "host"
	logStage   = "stage"
)

// 로그 파일 로테이션 기본값
const (
	defaultMaxSize    = 10
	defaultMaxAge     = 10
	defaultMaxBackups = 10
	dateTimeFormat    = "2006-01-02 15:04:05.000"
)

func Panic() *zerolog.Event {
	return log.Panic().
		Str(logProject, app.GetParam().GetProject()).
		Str(logHost, app.GetParam().GetHostname()).
		Str(logStage, app.GetParam().GetStage())
}

func Fatal() *zerolog.Event {
	return log.Fatal().
		Str(logProject, app.GetParam().GetProject()).
		Str(logHost, app.GetParam().GetHostname()).
		Str(logStage, app.GetParam().GetStage())
}

func Error() *zerolog.Event {
	return log.Error().
		Str(logProject, app.GetParam().GetProject()).
		Str(logHost, app.GetParam().GetHostname()).
		Str(logStage, app.GetParam().GetStage())
}

func Warn() *zerolog.Event {
	return log.Warn().
		Str(logProject, app.GetParam().GetProject()).
		Str(logHost, app.GetParam().GetHostname()).
		Str(logStage, app.GetParam().GetStage())
}

func Info() *zerolog.Event {
	return log.Info().
		Str(logProject, app.GetParam().GetProject()).
		Str(logHost, app.GetParam().GetHostname()).
		Str(logStage, app.GetParam().GetStage())
}

func Debug() *zerolog.Event {
	return log.Debug().
		Str(logProject, app.GetParam().GetProject()).
		Str(logHost, app.GetParam().GetHostname()).
		Str(logStage, app.GetParam().GetStage())
}

func Trace() *zerolog.Event {
	return log.Trace().
		Str(logProject, app.GetParam().GetProject()).
		Str(logHost, app.GetParam().GetHostname()).
		Str(logStage, app.GetParam().GetStage())
}

func SetCallerMarshalFunc() {
	zerolog.CallerMarshalFunc = func(pc uintptr, file string, line int) string {
		f := filepath.Base(file)
		return f + ":" + strconv.Itoa(line)
	}
}

func SetLogLevel(level string) {
	if logLevel, err := zerolog.ParseLevel(strings.ToLower(level)); err == nil {
		zerolog.SetGlobalLevel(logLevel)
	}
}

func SetLogWriters(out []string, isJson bool) {
	var writers []io.Writer
	if len(out) > 0 {
		for _, o := range out {
			if o == "stdout" {
				if isJson {
					writers = append(writers, os.Stdout)
				} else {
					o := zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: dateTimeFormat, NoColor: false}
					writers = append(writers, o)
				}
			} else if o == "stderr" {
				writers = append(writers, zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: dateTimeFormat, NoColor: false})
			} else {
				l := &lumberjack.Logger{
					Filename:   o,
					MaxSize:    defaultMaxSize,
					MaxAge:     defaultMaxAge,
					MaxBackups: defaultMaxBackups,
					LocalTime:  true,
					Compress:   true,
				}
				_ = l.Rotate()
				writers = append(writers, l)
			}
		}
	} else {
		o := zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: dateTimeFormat}
		writers = append(writers, o)
	}

	log.Logger = log.With().Caller().Logger()
	if len(writers) > 0 {
		log.Logger = log.Output(io.MultiWriter(writers...))
	}
}
