package driver

import (
	"strings"
	"time"
)

const (
	OperationModeDryFloor OperationMode = "dry-floor"
	OperationModeHeating = "heating"
	OperationModeAntiFreeze = "antifreeze"
	OperationModeCooling = "cooling"
	OperationModeIdle = "idle"
	OperationModeLegionella = "legionella"
	OperationModeHoliday = "holiday"
	OperationModeProhibited = "prohibited"
)

type OperationMode string

type MitsubishiTime time.Time

func (t *MitsubishiTime) UnmarshalJSON(b []byte) error {
	value := strings.Trim(string(b), `"`)
	if value == "" || value == "null" {
		return nil
	}

	date, err := time.Parse("2006-01-02T15:04:05", value)
	if err != nil {
		return err
	}

	*t = MitsubishiTime(date)

	return nil
}
