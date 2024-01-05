package FlareData

import (
	"os"
	"testing"
)

func TestAppConfig(t *testing.T) {

	filePath := getConfigPath("config")
	os.Remove(filePath)

	data := loadAppConfigFromYaml("config")
	if data.Title != "flare" {
		t.Fatal("Load App Config Failed")
	}
	ok := saveAppConfigToYamlFile("config", data)
	if !ok {
		t.Fatal("Save App Config Failed")
	}

	os.Remove(filePath)
}
