package main

import (
	"net/http"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"rbf.dev/melcloud_prometheus_exporter/config"
	"rbf.dev/melcloud_prometheus_exporter/driver"
	"rbf.dev/melcloud_prometheus_exporter/driver/ecodan"
	"rbf.dev/melcloud_prometheus_exporter/melcloud"
)

var (
    reg = prometheus.NewRegistry()
    ecodanStatsManager = ecodan.NewDefaultStatsManager()
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
    config, err := config.Parse(configPath)
    if err != nil {
        log.Error().Err(err).Msg("Unable to parse config")
    }

    requestor, err := melcloud.Authenticate(config.MELCloudConfig.Mail, config.MELCloudConfig.Password)
    if err != nil {
        log.Fatal().Err(err).Msg("Unable to authenticate with MELCloud")
    }

    initialFetchCh := make(chan bool)

    go fetchStats(requestor, config.Devices, initialFetchCh)

    log.Info().Msg("Waiting for initial statistics fetch to succeed...")

    if !<-initialFetchCh {
        log.Fatal().Msg("Initial fetch failed")
    }

    log.Info().
        Str("ListenAddress", config.ListenAddress).
        Msg("Initial fetch completed successfully, registering metrics and starting Prometheus server")

    melcloudRegisterer := prometheus.WrapRegistererWithPrefix("melcloud_", reg)
    ecodan.RegisterCollector(ecodanStatsManager, melcloudRegisterer)

    http.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
    http.ListenAndServe(config.ListenAddress, nil)
}

func fetchStats(
    requestor *melcloud.MelcloudRequestor,
    devices []config.MELCloudDeviceDescriptor,
    initialFetchCh chan bool,
) {
    didCompleteInitialFetch := false
    nextCommunicationDates := make([]time.Time, 0, len(devices))

    for {
        var err error

        deviceLoop: for _, descriptor := range devices {
            log.Debug().Str("Label", descriptor.Label).Msg("Fetching statistics for device")

            reader, err := requestor.GetDeviceInformation(descriptor.Id, descriptor.BuildingId)

            if err != nil {
                log.Error().
                    Err(err).
                    Str("Label", descriptor.Label).
                    Str("DeviceType", descriptor.Type).
                    Str("DeviceID", descriptor.Id).
                    Str("BuildingID", descriptor.BuildingId).
                    Msg("Failed to fetch statistics")
                if !didCompleteInitialFetch {
                    break
                }
                continue
            }

            defer reader.Close()

            var update *driver.Update

            switch descriptor.Type {
            case config.DeviceTypeEcodan:
                update, err = ecodanStatsManager.ParseAndUpdateStats(reader)

                if err != nil {
                    log.Error().
                        Err(err).
                        Str("Label", descriptor.Label).
                        Str("DeviceType", descriptor.Type).
                        Str("DeviceID", descriptor.Id).
                        Str("BuildingID", descriptor.BuildingId).
                        Msg("Failed to decode Ecodan model from statistics")
                    if !didCompleteInitialFetch {
                        break deviceLoop
                    }
                    continue deviceLoop
                }

                break
            default:
                log.Panic().Str("DeviceType", descriptor.Type).Msg("Unknown device type")
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
            // Try again in one minute if no attempt succeeded.
            <- time.After(time.Minute)
            continue
        }

        var maxDate time.Time

        for _, date := range nextCommunicationDates {
            if date.After(maxDate) {
                maxDate = date
            }
        }

        nextCommunicationDates = nextCommunicationDates[:0]

        maxDate = maxDate.Add(5 * time.Second) // 5 seconds buffer

        log.Debug().Time("NextTick", maxDate).Msg("Waiting until next tick to perform next statistics fetch")

        <- time.After(time.Until(maxDate))
    }}
