package domain

import (
	"errors"
	"strconv"
	"strings"
)

type SensorData struct {
	Temperature     float64 `json:"temperature"`
	Humidity        float64 `json:"humidity"`
	WateringEnabled bool    `json:"watering"`
	FanEnabled      bool    `json:"fan"`
}

var (
	ErrInvalidSensorData = errors.New("invalid sensor data")
)

func NewSensorDataFromRawData(rawData []byte) (*SensorData, error) {
	strData := string(rawData)
	dataTokens := strings.Split(strData, ",")
	if len(dataTokens) != 5 {
		return nil, ErrInvalidSensorData
	}
	if dataTokens[0] != "1" {
		return nil, ErrInvalidSensorData
	}
	temperature, ok := strconv.ParseFloat(dataTokens[1], 64)
	if ok != nil {
		return nil, ErrInvalidSensorData
	}
	humidity, ok := strconv.ParseFloat(dataTokens[2], 64)
	if ok != nil {
		return nil, ErrInvalidSensorData
	}
	wateringEnabled, ok := strconv.ParseBool(dataTokens[4])
	if ok != nil {
		return nil, ErrInvalidSensorData
	}
	wateringEnabled = !wateringEnabled
	fanEnabled, ok := strconv.ParseBool(dataTokens[3])
	if ok != nil {
		return nil, ErrInvalidSensorData
	}
	fanEnabled = !fanEnabled

	return &SensorData{
		Temperature:     temperature,
		Humidity:        humidity,
		WateringEnabled: wateringEnabled,
		FanEnabled:      fanEnabled,
	}, nil
}
