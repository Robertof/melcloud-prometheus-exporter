package main

import (
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"rbf.dev/melcloud_prometheus_exporter/config"
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

    _ = config
}
