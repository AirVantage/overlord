package main

import (
	"log/slog"
	"net"
	"os"

	"github.com/dusted-go/logging/prettylog"
	slogmulti "github.com/samber/slog-multi"
	slogsyslog "github.com/samber/slog-syslog/v2"
)

func InitLog() {
	var (
		syslogCfg string
		handlers  []slog.Handler
	)

	loggerLevel := slog.LevelInfo
	if *verboseLog {
		loggerLevel = slog.LevelDebug
	}

	// Always add stdout handler
	handlers = append(handlers, prettylog.New(
		&slog.HandlerOptions{Level: loggerLevel},
		prettylog.WithDestinationWriter(os.Stdout),
	))

	// Add syslog handler if configured
	syslogCfg = os.Getenv("SYSLOG_ADDRESS")
	if len(syslogCfg) > 0 {
		writer, err := net.Dial("udp", syslogCfg)
		if err != nil {
			slog.Warn("Unable to establish UDP session, cannot send logs to syslog", "error", err)
		} else {
			handlers = append(handlers, slogsyslog.Option{Level: loggerLevel, Writer: writer}.NewSyslogHandler())
		}
	}

	// Create a multi handler that writes to all configured handlers
	logger := slog.New(slogmulti.Fanout(handlers...))
	logger = logger.
		With("app_name", "overlord").
		With("app_version", Version)

	slog.SetDefault(logger)
}
