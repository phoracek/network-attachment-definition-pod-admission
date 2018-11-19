package admission

import (
	"fmt"
	"io/ioutil"

	"gopkg.in/yaml.v2"
)

type Config struct {
	Rules []Rule `json:"rules"`
}

type Rule struct {
	Type  string `json:"type"`
	Patch string `json:"patch"`
}

func LoadConfig(configFile string) (*Config, error) {
	yamlFile, err := ioutil.ReadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("unable to read config file: %v", err)
	}

	var config Config
	err = yaml.Unmarshal(yamlFile, &config)
	if err != nil {
		return nil, fmt.Errorf("unable to parse configuration: %v", err)
	}

	return &config, nil
}
