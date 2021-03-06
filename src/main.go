package main

import (
	"net/http"
    // _ "net/http/pprof"
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
        log.Error().Err(err).Msg("Unable to parse config")
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
    nextCommunicationDates := make([]time.Time, 0, len(devices))

    for {
        var err error

        for _, descriptor := range devices {
            log.Debug().Str("Label", descriptor.Label).Msg("Fetching statistics for device")

            reader, err := requestor.GetDeviceInformation(descriptor.Id, descriptor.BuildingId)

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
                if !didCompleteInitialFetch {
                    break
                }
                continue
            }

            statsManager := statsManagers[descriptor.Label]
            update, err := statsManager.ParseAndUpdateStats(reader)
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

        nextCommunicationDates = make([]time.Time, 0, len(devices))

        maxDate = maxDate.Add(5 * time.Second) // 5 seconds buffer

        log.Debug().Time("NextTick", maxDate).Msg("Waiting until next tick to perform next statistics fetch")

        <- time.After(time.Until(maxDate))
    }}
