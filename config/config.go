package config

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
)

var Golbal *Config

type Config struct {
	ROOT_PATH     string `json:"ONEBLOG_ROOT_PATH"`
	CLIENT_ID     string `json:"ONEBLOG_CLIENT_ID"`
	CLIENT_SECRET string `json:"ONEBLOG_CLIENT_SECRET"`
	REDIRECT_URI  string `json:"ONEBLOG_REDIRECT_URI"`
	RATE_LIMIT    int    `json:"ONEBLOG_RATE_LIMIT"`
}

func Load() error {
	configFile, err := os.Open("config.json")
	if err != nil {
		return err
	}
	defer configFile.Close()

	jsonParser := json.NewDecoder(configFile)
	if err := jsonParser.Decode(&Golbal); err != nil {
		return err
	}

	v := reflect.ValueOf(Golbal).Elem()
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		fieldVal := v.Field(i)
		jsonTag := field.Tag.Get("json")

		if _, ok := os.LookupEnv(jsonTag); !ok {
			continue
		}
		switch fieldVal.Kind() {
		case reflect.String:
			fieldVal.SetString(os.Getenv(jsonTag))
		case reflect.Int:
			var value int64
			fmt.Sscanf(os.Getenv(jsonTag), "%d", &value)
			fieldVal.SetInt(value)
		}
	}
	return nil
}
