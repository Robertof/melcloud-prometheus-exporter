package config

import (
    "encoding/json"
    "fmt"
    "os"
)

func Parse(path string) (*Config, error) {
    c, err := os.Open(path)
    if err != nil {
        return nil, fmt.Errorf("Unable to open config file for reading: %w", err)
    }

    defer c.Close()

    cfg := Config{}
    d := json.NewDecoder(c)
    if err = d.Decode(&cfg); err != nil {
        return nil, fmt.Errorf("Unable to decode config file: %w", err)
    }

    return &cfg, nil
}
