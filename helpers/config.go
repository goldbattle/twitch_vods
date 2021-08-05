package helpers

import (
	"../models"
	"encoding/json"
	"io/ioutil"
	"log"
)

func LoadConfigFile(configPath string) models.ConfigurationFile {
	file, err := ioutil.ReadFile(configPath)
	if err != nil {
		log.Fatalf("CONFIG: error loading config file %s\nCONFIG: %s\n", configPath, err)
	}
	config := models.ConfigurationFile{}
	err = json.Unmarshal(file, &config)
	if err != nil {
		log.Fatalf("CONFIG: error loading config file %s\nCONFIG: %s\n", configPath, err)
	}
	return config
}
