package driver

import (
	"fmt"
	"strings"
	"time"
)

const (
    OperationModeDryFloor OperationMode = iota
    OperationModeHeating
    OperationModeAntiFreeze
    OperationModeCooling
    OperationModeIdle
    OperationModeLegionella
    OperationModeHoliday
    OperationModeProhibited
)

type OperationMode uint

//go:generate enumer -type=OperationMode

func OperationModeHelpString() string {
    out := make([]string, len(OperationModeValues()))

    for index, op := range OperationModeValues() {
        out[index] = fmt.Sprintf("%v (%v)", int(op), op.String()[len("OperationMode"):])
    }

    return "Available values: " + strings.Join(out, ", ")
}

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

type Update struct {
    // Suggested timestamp representing when the next communication should occur.
    NextCommunication time.Time
}
