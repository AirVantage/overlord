package main

import (
	"log/slog"
	"log/syslog"
	"os"

	"github.com/dusted-go/logging/prettylog"
	slogmulti "github.com/samber/slog-multi"
	slogsyslog "github.com/samber/slog-syslog/v2"
)

// customConverter removes logger.name and logger.version from syslog output
func customConverter(addSource bool, replaceAttr func(groups []string, a slog.Attr) slog.Attr, loggerAttr []slog.Attr, groups []string, record *slog.Record) map[string]any {
	// Get the default conversion
	attrs := slogsyslog.DefaultConverter(addSource, replaceAttr, loggerAttr, groups, record)

	// Remove logger_name and logger_version
	delete(attrs, "logger.name")
	delete(attrs, "logger.version")

	// Convert timestamp to ISO 8601 format for loggly
	attrs["timestamp"] = record.Time.UTC().Format("2006-01-02T15:04:05.999Z07:00")

	return attrs
}

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
		writer, err := syslog.Dial("udp", syslogCfg, syslog.LOG_INFO|syslog.LOG_DAEMON, "overlord")
		if err != nil {
			slog.Warn("Unable to establish UDP session, cannot send logs to syslog", "error", err)
		} else {
			handlers = append(handlers, slogsyslog.Option{
				Level:     loggerLevel,
				Writer:    writer,
				Converter: customConverter,
			}.NewSyslogHandler())
		}
	}

	// Create a multi handler that writes to all configured handlers
	logger := slog.New(slogmulti.Fanout(handlers...))
	slog.SetDefault(logger)
}
