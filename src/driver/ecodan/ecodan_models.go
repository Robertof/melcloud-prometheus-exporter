package ecodan

import (
    "encoding/json"

    "rbf.dev/melcloud_prometheus_exporter/driver"
)

type EcodanStatistics struct {
    OperationMode driver.OperationMode `json:"-"`
    OperationModeZone1 driver.OperationMode `json:"-"`

    SetHeatFlowTemperatureZone1 float32
    ProhibitZone1 bool

    SetTankWaterTemperature float32
    TankWaterTemperature float32
    ProhibitHotWater bool

    ForcedHotWaterMode bool
    OutdoorTemperature float32
    LastCommunication driver.MitsubishiTime
    NextCommunication driver.MitsubishiTime
    HolidayMode, Power, Offline bool

    RawOperationMode int `json:"OperationMode"`
    RawOperationModeZone1 int `json:"OperationModeZone1"`
}

func (stats *EcodanStatistics) UnmarshalJSON(b []byte) error {
    type rawStats *EcodanStatistics
    if err := json.Unmarshal(b, rawStats(stats)); err != nil {
        return err
    }

    // Adapt operation mode.
    opMode, zone1OpMode := stats.RawOperationMode, stats.RawOperationModeZone1

    func() {
        if stats.HolidayMode {
            stats.OperationMode = driver.OperationModeHoliday
            stats.OperationModeZone1 = driver.OperationModeHoliday
            return
        }

        // Zone 1
        if stats.ProhibitZone1 {
            stats.OperationModeZone1 = driver.OperationModeProhibited
        } else if opMode == 2 {
            if zone1OpMode == 5 {
                stats.OperationModeZone1 = driver.OperationModeDryFloor
            } else {
                stats.OperationModeZone1 = driver.OperationModeHeating
            }
        } else if opMode == 5 {
            stats.OperationModeZone1 = driver.OperationModeAntiFreeze
        } else if opMode == 3 {
            stats.OperationModeZone1 = driver.OperationModeCooling
        } else {
            stats.OperationModeZone1 = driver.OperationModeIdle
        }

        // Hot water
        if stats.ProhibitHotWater {
            stats.OperationMode = driver.OperationModeProhibited
        } else if opMode == 1 {
            stats.OperationMode = driver.OperationModeHeating
        } else if opMode == 6 {
            stats.OperationMode = driver.OperationModeLegionella
        } else {
            stats.OperationMode = driver.OperationModeIdle
        }
    }()

    return nil
}
