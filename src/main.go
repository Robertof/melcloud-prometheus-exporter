package main

import (
	"errors"
	"io"
	"math"
	"net/http"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/robertof/go-melcloud"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"rbf.dev/melcloud_prometheus_exporter/config"
	"rbf.dev/melcloud_prometheus_exporter/driver"
	"rbf.dev/melcloud_prometheus_exporter/driver/ecodan"
)

var (
    reg = prometheus.NewRegistry()
    statsManagers = make(map[string]driver.StatsManager)
)

func main() {
    if os.Getenv("MELCLOUD_PROMETHEUS_EXPORTER_DEBUG") != "" {
        zerolog.SetGlobalLevel(zerolog.DebugLevel)
    } else if os.Getenv("MELCLOUD_PROMETHEUS_EXPORTER_TRACE") != "" {
        zerolog.SetGlobalLevel(zerolog.TraceLevel)
    } else {
        zerolog.SetGlobalLevel(zerolog.InfoLevel)
    }

    log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

    if len(os.Args) < 2 {
        log.Fatal().Msgf("Usage: %v <config-path>", os.Args[0])
    }

    configPath := os.Args[1]

    log.Debug().Str("path", configPath).Msg("Parsing configuration")

    // Parse the configuration.
    cfg, err := config.Parse(configPath)
    if err != nil {
        log.Fatal().Err(err).Msg("Unable to parse config")
    }

    requestor, err := melcloud.Authenticate(cfg.MELCloudConfig.Mail, cfg.MELCloudConfig.Password)
    if err != nil {
        log.Fatal().Err(err).Msg("Unable to authenticate with MELCloud")
    }

    log.Info().Msg("Bootstrapping statistics managers...")

    melcloudRegisterer := prometheus.WrapRegistererWithPrefix("melcloud_", reg)

    for _, descriptor := range cfg.Devices {
        if _, ok := statsManagers[descriptor.Label]; ok {
            log.Panic().Str("Label", descriptor.Label).Msg("Duplicated device labels are not permitted!")
        }

        reg := prometheus.WrapRegistererWith(prometheus.Labels{
            "device": descriptor.Label,
        }, melcloudRegisterer)

        switch descriptor.Type {
        case config.DeviceTypeEcodan:
            manager := ecodan.NewDefaultStatsManager()
            statsManagers[descriptor.Label] = manager
            manager.RegisterMetrics(reg)
            break
        default:
            log.Panic().Str("DeviceType", string(descriptor.Type)).Msg("Unknown device type")
        }
    }

    initialFetchCh := make(chan bool)

    go fetchStats(requestor, cfg.Devices, initialFetchCh)

    log.Info().Msg("Waiting for initial statistics fetch to succeed...")

    if !<-initialFetchCh {
        log.Fatal().Msg("Initial fetch failed")
    }

    log.Info().
        Str("ListenAddress", cfg.ListenAddress).
        Msg("Initial fetch completed successfully, starting Prometheus server")

    http.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
    http.ListenAndServe(cfg.ListenAddress, nil)
}

func fetchStats(
    requestor *melcloud.MelcloudRequestor,
    devices []config.MELCloudDeviceDescriptor,
    initialFetchCh chan bool,
) {
    didCompleteInitialFetch := false
    backoffFactor := 1
    nextCommunicationDates := make([]time.Time, 0, len(devices))

    for {
        var err error

        for _, descriptor := range devices {
            log.Debug().Str("Label", descriptor.Label).Msg("Fetching statistics for device")

            var reader io.ReadCloser
            reader, err = requestor.GetDeviceInformation(descriptor.Id, descriptor.BuildingId)

            if err != nil {
                log.Error().
                    Err(err).
                    Str("Label", descriptor.Label).
                    Str("DeviceType", string(descriptor.Type)).
                    Str("DeviceID", descriptor.Id).
                    Str("BuildingID", descriptor.BuildingId).
                    Msg("Failed to fetch statistics")
                if reader != nil {
                    reader.Close()
                }
                // if we are ratelimited, do not attempt to issue requests for other devices.
                if !didCompleteInitialFetch || errors.Is(err, melcloud.ErrTooManyRequests) {
                    break
                }
                continue
            }

            statsManager := statsManagers[descriptor.Label]
            var update *driver.Update
            update, err = statsManager.ParseAndUpdateStats(reader)
            reader.Close()

            if err != nil {
                log.Error().
                    Err(err).
                    Str("Label", descriptor.Label).
                    Str("DeviceType", string(descriptor.Type)).
                    Str("DeviceID", descriptor.Id).
                    Str("BuildingID", descriptor.BuildingId).
                    Msg("Failed to decode model from statistics")
                if !didCompleteInitialFetch {
                    break
                }
                continue
            }

            if update != nil {
                nextCommunicationDates = append(nextCommunicationDates, update.NextCommunication)
            }
        }

        if !didCompleteInitialFetch {
            if err == nil {
                didCompleteInitialFetch = true
            }

            initialFetchCh <- err == nil

            if err != nil {
                return
            }
        }

        // Determine the farthest communication date.
        if len(nextCommunicationDates) == 0 {
            // Try again based on the backoff factor if no attempt succeeded.
            wait := time.Minute * time.Duration(math.Pow(2, float64(backoffFactor)))
            if backoffFactor < 4 {
                // limit to 16 mins (2**4) backoff.
                backoffFactor += 1
            }
            log.Debug().Dur("Backoff", wait).Msg("No attempt succeeded - backing off")
            <- time.After(wait)
            continue
        }

        // reset backoff factor after any fetch succeeded.
        backoffFactor = 1
        var maxDate time.Time

        for _, date := range nextCommunicationDates {
            if date.After(maxDate) {
                maxDate = date
            }
        }

        nextCommunicationDates = make([]time.Time, 0, len(devices))

        if maxDate.IsZero() || time.Until(maxDate) < 1 * time.Minute {
            // enforce a minimum wait of 1m10s in case the date is in the past (likely due
            // to some sort of time skew).
            maxDate = time.Now().Add(1 * time.Minute + 10 * time.Second)
            log.Debug().
                Stringer("NextTick", maxDate).
                Msg("Ignoring suggested next tick as it's zero or too small - using 1m10s")
        } else if time.Until(maxDate) > 5 * time.Minute {
            // do not bother waiting more than 5 minutes for an update.
            maxDate = time.Now().Add(5 * time.Minute)
            log.Debug().
                Stringer("NextTick", maxDate).
                Msg("Ignoring suggested next tick as it's over 5 minutes - using 5m")
        }

        log.Debug().Time("NextTick", maxDate).Msg("Waiting until next tick to perform next statistics fetch")

        <- time.After(time.Until(maxDate))
    }}
