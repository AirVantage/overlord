package main

import (
	"log/slog"
	"net"
	"os"

	"github.com/dusted-go/logging/prettylog"
	slogsyslog "github.com/samber/slog-syslog/v2"
)

func InitLog() {
	var (
		syslogCfg string
	)

	loggerLevel := slog.LevelInfo
	if *verboseLog {
		loggerLevel = slog.LevelDebug
	}

	handler := prettylog.New(
		&slog.HandlerOptions{Level: loggerLevel},
		prettylog.WithDestinationWriter(os.Stdout),
	)

	slog.SetDefault(slog.New(handler))

	syslogCfg = os.Getenv("SYSLOG_ADDRESS")
	if len(syslogCfg) > 0 {
		writer, err := net.Dial("udp", syslogCfg)
		if err != nil {
			slog.Warn("Unable to establish UDP session, cannot send logs to syslog", "error", err)
		} else {
			handler := slogsyslog.Option{Level: loggerLevel, Writer: writer}.NewSyslogHandler()
			slog.SetDefault(slog.New(handler))
		}
	}
}
