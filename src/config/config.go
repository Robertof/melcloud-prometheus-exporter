package config

type DeviceType string

const (
    DeviceTypeEcodan DeviceType = "ecodan"
)

type Config struct {
    ListenAddress string `default:"localhost:9102"`
    MELCloudConfig MELCloudConfig
    Devices []MELCloudDeviceDescriptor
}

type MELCloudConfig struct {
    Mail, Password string
}

type MELCloudDeviceDescriptor struct {
    Type DeviceType
    Label, Id, BuildingId string
}
