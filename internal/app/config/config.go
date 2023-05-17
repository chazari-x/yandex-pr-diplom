package config

import (
	"flag"
	"log"

	"github.com/caarlos0/env/v6"
	_ "github.com/lib/pq"
)

var C Config

type Config struct {
	RunAddress           string `env:"RUN_ADDRESS"`
	AccrualSystemAddress string `env:"ACCRUAL_SYSTEM_ADDRESS"`
	DataBaseURI          string `end:"DATABASE_URI"`
}

//var f flagConfig

//type flagConfig struct {
//	RunAddress           *string
//	AccrualSystemAddress *string
//	DataBaseURI          *string
//}

//
//func init() {
//	f.RunAddress = flag.String("a", "asd", "run address")
//	f.AccrualSystemAddress = flag.String("r", "", "accrual system address")
//	f.DataBaseURI = flag.String("d", "", "database uri")
//}

func GetConfig() (Config, error) {
	err := env.Parse(&C)
	if err != nil {
		return Config{}, err
	}

	flag.StringVar(&C.RunAddress, "a", C.RunAddress, "run address")
	flag.StringVar(&C.AccrualSystemAddress, "r", C.AccrualSystemAddress, "accrual system address")
	flag.StringVar(&C.DataBaseURI, "d", C.DataBaseURI, "database uri")
	flag.Parse()
	//if C.RunAddress == "" {
	//	C.RunAddress = "localhost:8080"
	//}
	//
	////if C.AccrualSystemAddress == "" {
	////	return Config{}, errors.New("accrual system address is nil")
	////}

	log.Print(C)

	return C, nil
}
