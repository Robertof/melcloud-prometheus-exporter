package ecodan

import (
  "time"

  "github.com/prometheus/client_golang/prometheus"
  "rbf.dev/melcloud_prometheus_exporter/driver"
)

var (
  descOperationMode = prometheus.NewDesc(
    "ecodan_operation_mode",
    "Operation mode for the whole ECODan machine. " + driver.OperationModeHelpString(),
    nil,
    nil,
  )
  descOperationModeZone = prometheus.NewDesc(
    "ecodan_zone_operation_mode",
    "Operation mode for individual ECODan zones." + driver.OperationModeHelpString(),
    []string{"zone_number"},
    nil,
  )
  descHeatFlowTemperatureSetpoint = prometheus.NewDesc(
    "ecodan_heat_flow_temperature_setpoint_celsius",
    "Heat flow temperature setpoint for individual ECODan zones.",
    []string{"zone_number"},
    nil,
  )
  descTankWaterTemperatureSetpoint = prometheus.NewDesc(
    "ecodan_tank_temperature_setpoint_celsius",
    "Tank temperature setpoint for the ECODan.",
    nil,
    nil,
  )
  descTankWaterTemperature = prometheus.NewDesc(
    "ecodan_tank_temperature_celsius",
    "Tank temperature for the ECODan.",
    nil,
    nil,
  )
  descForcedHotWater = prometheus.NewDesc(
    "ecodan_forced_hot_water_on",
    "Whether forced hot water mode is ON for the ECODan.",
    nil,
    nil,
  )
  descOutdoorTemperature = prometheus.NewDesc(
    "ecodan_outdoor_temperature_celsius",
    "Outdoor temperature retrieved by the ECODan.",
    nil,
    nil,
  )
  descPower = prometheus.NewDesc(
    "ecodan_power_on",
    "Power state of the ECODan.",
    nil,
    nil,
  )
  descOffline = prometheus.NewDesc(
    "ecodan_offline",
    "Whether the ECODan is offline.",
    nil,
    nil,
  )
  allDescriptors = []*prometheus.Desc{
    descOperationMode,
    descOperationModeZone,
    descHeatFlowTemperatureSetpoint,
    descTankWaterTemperatureSetpoint,
    descTankWaterTemperature,
    descForcedHotWater,
    descOutdoorTemperature,
    descPower,
    descOffline,
  }

)

type StatsProvider interface {
  Stats() (*EcodanStatistics, time.Time)
}

type collector struct {
  provider StatsProvider
}

func toBool(v bool) float64 {
  if v {
    return 1
  }
  return 0
}

func sendWithTimestamp(ch chan<- prometheus.Metric, t time.Time, m prometheus.Metric) {
  ch <- prometheus.NewMetricWithTimestamp(t, m)
}

func (collector collector) Describe(ch chan<- *prometheus.Desc) {
  for _, desc := range allDescriptors {
    ch <- desc
  }
}

func (collector collector) Collect(ch chan<- prometheus.Metric) {
  stats, t := collector.provider.Stats()

  if stats == nil {
    return
  }

  sendWithTimestamp(ch, t, prometheus.MustNewConstMetric(descOperationMode, prometheus.GaugeValue, float64(stats.OperationMode)))
  sendWithTimestamp(ch, t, prometheus.MustNewConstMetric(descOperationModeZone, prometheus.GaugeValue, float64(stats.OperationModeZone1), "1"))
  sendWithTimestamp(ch, t, prometheus.MustNewConstMetric(descHeatFlowTemperatureSetpoint, prometheus.GaugeValue, float64(stats.SetHeatFlowTemperatureZone1), "1"))
  sendWithTimestamp(ch, t, prometheus.MustNewConstMetric(descTankWaterTemperatureSetpoint, prometheus.GaugeValue, float64(stats.SetTankWaterTemperature)))
  sendWithTimestamp(ch, t, prometheus.MustNewConstMetric(descTankWaterTemperature, prometheus.GaugeValue, float64(stats.TankWaterTemperature)))
  sendWithTimestamp(ch, t, prometheus.MustNewConstMetric(descForcedHotWater, prometheus.GaugeValue, toBool(stats.ForcedHotWaterMode)))
  sendWithTimestamp(ch, t, prometheus.MustNewConstMetric(descOutdoorTemperature, prometheus.GaugeValue, float64(stats.OutdoorTemperature)))
  sendWithTimestamp(ch, t, prometheus.MustNewConstMetric(descPower, prometheus.GaugeValue, toBool(stats.Power)))
  sendWithTimestamp(ch, t, prometheus.MustNewConstMetric(descOffline, prometheus.GaugeValue, toBool(stats.Offline)))
}

func RegisterCollector(provider StatsProvider, reg prometheus.Registerer) {
  collector := collector{provider}
  reg.MustRegister(collector)
}
