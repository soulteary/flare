package FlareData

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestCheckExists(t *testing.T) {

	exist := checkExists("NOT_EXIST")
	if exist != false {
		t.Fatal("check exist failed")
	}

	workDir, _ := os.Getwd()
	exist = checkExists(workDir)
	if exist != true {
		t.Fatal("check exist failed")
	}

}

func TestSaveAndReadFile(t *testing.T) {

	workDir, _ := os.Getwd()
	filePath := filepath.Join(workDir, "test.yml")
	content := []byte("test")

	ok := saveFile(filePath, content)

	if !ok {
		t.Fatal("save file failed")
	}

	data := readFile(filePath, false)

	res := bytes.Compare(content, data)
	if res != 0 {
		t.Fatal("read file failed")
	}

	os.Remove(filePath)
}
