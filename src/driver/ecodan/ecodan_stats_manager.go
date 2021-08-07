package ecodan

import (
	"encoding/json"
	"io"
	"sync"
	"time"

	"rbf.dev/melcloud_prometheus_exporter/driver"
)

type StatsManager struct {
	mu sync.RWMutex
	lastStats *EcodanStatistics
}

func NewDefaultStatsManager() *StatsManager {
	return &StatsManager{}
}

func (s *StatsManager) updateStats(stats *EcodanStatistics) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.lastStats = stats
}

func (s *StatsManager) ParseAndUpdateStats(reader io.ReadCloser) (*driver.Update, error) {
    var statistics EcodanStatistics

    if err := json.NewDecoder(reader).Decode(&statistics); err != nil {
    	return nil, err
    }

    s.updateStats(&statistics)

    return &driver.Update{
    	NextCommunication: time.Time(statistics.NextCommunication),
    }, nil
}

func (s *StatsManager) Stats() *EcodanStatistics {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Safe to unlock after returning the current value of the `lastStats` pointer as the
	// stats object is never mutated, the pointer is just swapped around.
	return s.lastStats
}
