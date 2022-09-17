package builder

import (
	"encoding/json"
	"fmt"
	"go/format"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

func TaskForMdi(src string, dest string, res string, gofile string) {
	initMdiResourceTemplate(res, gofile)
	_PrepareDirectory(dest)
	if err := _CopyDirectoryWithoutSymlink(src, dest); err != nil {
		log.Fatal(err)
	}
}

func initMdiResourceTemplate(src string, dest string) {
	// https://www.npmjs.com/package/@mdi/js
	file := src
	fileRaw, err := os.ReadFile(filepath.Clean(file))
	mdiJSON := ""
	if err != nil {
		fmt.Println("读取文件出错", file)
	} else {
		var re = regexp.MustCompile(`(?m)exports.mdi(\w+)\s*=\s*"(.+?)";`)

		icons := make(map[string]string)

		for _, match := range re.FindAllStringSubmatch(string(fileRaw), -1) {
			icons[strings.ToLower(match[1])] = match[2]
		}

		file, _ := json.MarshalIndent(icons, "", " ")
		mdiJSON = string(file)
	}

	goFile := "package mdi\nvar iconMap = map[string]string" + mdiJSON
	goFile = strings.Replace(goFile, "\"\n}", "\",\n}", 1)
	content, _ := format.Source([]byte(goFile))

	err = os.WriteFile(dest, content, os.ModePerm)
	if err != nil {
		fmt.Println("保存文件出错", err)
	} else {
		fmt.Println("保存 MDI 资源文件完毕", dest)
	}
}
