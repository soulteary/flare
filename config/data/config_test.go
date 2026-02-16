package data

import (
	"os"
	"testing"
)

func TestAppConfig(t *testing.T) {

	filePath := getConfigPath("config")
	os.Remove(filePath)

	data, err := loadAppConfigFromYaml("config")
	if err != nil {
		t.Fatalf("Load App Config: %v", err)
	}
	if data.Title != "flare" {
		t.Fatal("Load App Config Failed")
	}
	ok := saveAppConfigToYamlFile("config", data)
	if !ok {
		t.Fatal("Save App Config Failed")
	}

	os.Remove(filePath)
}
