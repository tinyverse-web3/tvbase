package config

import (
	"encoding/json"
	"os"
)

func LoadConfig(cfg any, filePath string) error {
	if filePath != "" {
		cfgFile, err := os.Open(filePath)
		if err != nil {
			return err
		}
		defer cfgFile.Close()

		decoder := json.NewDecoder(cfgFile)
		err = decoder.Decode(&cfg)
		if err != nil {
			return err
		}
	}
	return nil
}
