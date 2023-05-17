package config

import (
	"errors"
	"flag"

	"github.com/caarlos0/env/v6"
	_ "github.com/lib/pq"
)

var C Config

type Config struct {
	RunAddress           string `env:"RUN_ADDRESS"`
	AccrualSystemAddress string `env:"ACCRUAL_SYSTEM_ADDRESS"`
	DataBaseURI          string `env:"DATABASE_URI"`
}

func GetConfig() (Config, error) {
	err := env.Parse(&C)
	if err != nil {
		return Config{}, err
	}

	flag.StringVar(&C.RunAddress, "a", C.RunAddress, "run address")
	flag.StringVar(&C.AccrualSystemAddress, "r", C.AccrualSystemAddress, "accrual system address")
	flag.StringVar(&C.DataBaseURI, "d", C.DataBaseURI, "database uri")
	flag.Parse()

	if C.RunAddress == "" || C.AccrualSystemAddress == "" || C.DataBaseURI == "" {
		return Config{}, errors.New("error config")
	}

	return C, nil
}
