package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"time"

	"github.com/AirVantage/overlord/pkg/state"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsretry "github.com/aws/aws-sdk-go-v2/aws/retry"
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

	// Handle termination signals
	termSig := make(chan os.Signal, 1)
	signal.Notify(termSig, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)
	go func() {
		sig := <-termSig
		slog.Error("Received termination signal", "signal", sig)
		os.Exit(1)
	}()

	// Handle SIGHUP for configuration reload
	hupSig := make(chan os.Signal, 1)
	signal.Notify(hupSig, syscall.SIGHUP)

	flag.Parse()
	InitLog()

	slog.Info("overlord starting", "version", Version, "commit", Commit, "date", Time)

	// Initialise AWS SDK v2, process default configuration with retry configuration
	cfg, err = config.LoadDefaultConfig(ctx,
		config.WithRetryer(func() aws.Retryer {
			return awsretry.NewAdaptiveMode(func(o *awsretry.AdaptiveModeOptions) {
				// Configure standard retry options
				o.StandardOptions = []func(*awsretry.StandardOptions){
					func(so *awsretry.StandardOptions) {
						so.MaxAttempts = 10              // Increase max attempts
						so.MaxBackoff = 60 * time.Second // Increase max backoff time
						so.RetryCost = 1                 // Reduce retry cost to allow more retries
						so.RetryTimeoutCost = 2          // Reduce timeout retry cost
						so.NoRetryIncrement = 2          // Increase token payback for successful attempts
					},
				}
			})
		}),
	)
	if err != nil {
		slog.Error("unable to initialize AWS SDK v2", "detail", err)
		os.Exit(1)
	}

	// Main loop
	for {
		runningState, err = Iterate(ctx, cfg, runningState, hupSig)
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

			os.Exit(1)
		}

		// Sleep for the configured interval, but wake up immediately on SIGHUP
		select {
		case <-time.After(*interval):
			// Normal interval elapsed, continue to next iteration
		case <-hupSig:
			slog.Info("Received SIGHUP, interrupting sleep for immediate iteration")
			// SIGHUP received, skip remaining sleep time and iterate immediately
		}
	}
}
