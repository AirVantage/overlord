package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"

	"time"

	"github.com/AirVantage/overlord/pkg/state"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/smithy-go"
)

var (
	configRoot       = flag.String("etc", "/etc/overlord", "path to configuration directory")
	resourcesDirName = "resources"
	templatesDirName = "templates"
	interval         = flag.Duration("interval", 30*time.Second, "Interval between each lookup")
	ipv6             = flag.Bool("ipv6", false, "Look for IPv6 addresses instead of IPv4")
	verboseLog       = flag.Bool("v", false, "verbose debug information")
)

func main() {
	var (
		cfg          aws.Config
		ctx          context.Context = context.TODO()
		runningState *state.State    = state.New()
		err          error
	)

	flag.Parse()
	InitLog()

	slog.Info("overlord starting", "version", Version, "user", User, "date", Time)

	// Initialise AWS SDK v2, process default configuration
	cfg, err = config.LoadDefaultConfig(ctx)
	if err != nil {
		slog.Error("unable to initialize AWS SDK v2", "detail", err)
		return
	}

	// Main loop
	for {
		runningState, err = Iterate(ctx, cfg, runningState)
		if err != nil {
			var oe *smithy.OperationError
			var ae smithy.APIError

			if errors.As(err, &oe) {
				slog.Error("Failed service call processing ..", "service", oe.Service(), "operation", oe.Operation(), "error", oe.Unwrap().Error())
			} else {
				if errors.As(err, &ae) {
					slog.Error("AWS API Error detail", "code", ae.ErrorCode(), "message", ae.ErrorMessage(), "fault", ae.ErrorFault().String())
				} else {
					slog.Error(err.Error())
				}
			}

			return
		}
		time.Sleep(*interval)
	}
}
