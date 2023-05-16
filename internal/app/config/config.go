package config

import (
	"flag"

	"github.com/caarlos0/env/v6"
	_ "github.com/lib/pq"
)

var C Config

type Config struct {
	RunAddress           string `env:"RUN_ADDRESS"`
	AccrualSystemAddress string `env:"ACCRUAL_SYSTEM_ADDRESS"`
	DataBaseURI          string `end:"DATABASE_URI"`
}

var f flagConfig

type flagConfig struct {
	RunAddress           *string
	AccrualSystemAddress *string
	DataBaseURI          *string
}

func init() {
	f.RunAddress = flag.String("a", "localhost:8080", "run address")
	f.AccrualSystemAddress = flag.String("r", "", "accrual system address")
	f.DataBaseURI = flag.String("d", "", "database uri")
}

func GetConfig() (Config, error) {
	flag.Parse()
	C.RunAddress = *f.RunAddress
	C.AccrualSystemAddress = *f.AccrualSystemAddress
	C.DataBaseURI = *f.DataBaseURI

	err := env.Parse(&C)
	if err != nil {
		return Config{}, err
	}

	if C.RunAddress == "" {
		C.RunAddress = "localhost:8080"
	}

	return C, nil
}
