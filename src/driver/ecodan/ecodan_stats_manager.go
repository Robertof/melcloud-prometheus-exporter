package ecodan

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog/log"
	"rbf.dev/melcloud_prometheus_exporter/driver"
)

type statsManager struct {
    mu sync.RWMutex
    lastStats *EcodanStatistics
    lastUpdate time.Time
}

func NewDefaultStatsManager() driver.StatsManager {
    return &statsManager{}
}

func (s *statsManager) updateStats(stats *EcodanStatistics) {
    s.mu.Lock()
    defer s.mu.Unlock()

    s.lastStats = stats
    s.lastUpdate = time.Now()
}

func (s *statsManager) ParseAndUpdateStats(reader io.ReadCloser) (*driver.Update, error) {
    var statistics EcodanStatistics

    var buf strings.Builder
    tee := io.TeeReader(reader, &buf)

    if err := json.NewDecoder(tee).Decode(&statistics); err != nil {
        return nil, fmt.Errorf("while parsing '%.100v': %w", buf.String(), err)
    }

    log.Trace().
        Interface("Stats", statistics).
        Str("Raw", buf.String()).
        Msg("ecodan: successfully parsed statistics")

    s.updateStats(&statistics)

    return &driver.Update{
        NextCommunication: time.Time(statistics.NextCommunication),
    }, nil
}

func (s *statsManager) RegisterMetrics(reg prometheus.Registerer) {
    RegisterCollector(s, reg)
}

func (s *statsManager) Stats() (*EcodanStatistics, time.Time) {
    s.mu.RLock()
    defer s.mu.RUnlock()

    // Safe to unlock after returning the current value of the `lastStats` pointer as the
    // stats object is never mutated, the pointer is just swapped around.
    return s.lastStats, s.lastUpdate
}
