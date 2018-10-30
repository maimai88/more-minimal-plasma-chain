package main

import (
	"encoding/json"
	"flag"
	"os"

	"github.com/m0t0k1ch1/more-minimal-plasma-chain/app"
)

const (
	DefaultConfigPath = "config.json"
)

func loadConfig(path string) (*app.Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	var conf app.Config
	if err := json.NewDecoder(file).Decode(&conf); err != nil {
		return nil, err
	}

	return &conf, nil
}

func main() {
	var confPath = flag.String("conf", DefaultConfigPath, "path to your config.json")
	flag.Parse()

	conf, err := loadConfig(*confPath)
	if err != nil {
		panic(err)
	}

	cc, err := app.NewChildChain(conf)
	if err != nil {
		panic(err)
	}

	cc.Logger().Fatal(cc.Start())
}
