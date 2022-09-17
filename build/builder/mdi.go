package builder

import (
	"encoding/json"
	"fmt"
	"go/format"
	"io/ioutil"
	"log"
	"os"
	"regexp"
	"strings"
)

func TaskForMdi(src string, dest string, res string, gofile string) {
	initMdiResourceTemplate(res, "internal/mdi/icons.json", gofile)
	_PrepareDirectory(dest)
	_CopyDirectory(src, dest)
}

func initMdiResourceTemplate(src string, dest string, gofile string) {
	// https://www.npmjs.com/package/@mdi/js
	file := src
	fileRaw, err := ioutil.ReadFile(file)
	if err != nil {
		fmt.Println("读取文件出错", file)
	} else {
		var re = regexp.MustCompile(`(?m)exports.mdi(\w+)\s*=\s*"(.+?)";`)

		icons := make(map[string]string)

		for _, match := range re.FindAllStringSubmatch(string(fileRaw), -1) {
			icons[strings.ToLower(match[1])] = match[2]
		}

		file, _ := json.MarshalIndent(icons, "", " ")
		err = ioutil.WriteFile(dest, file, os.ModePerm)

		if err != nil {
			fmt.Println("保存文件出错", err)
		} else {
			fmt.Println("保存 MDI 文件完毕", dest)
		}
	}

	jsonRaw, err := ioutil.ReadFile(dest)
	if err != nil {
		log.Fatal(err)
		return
	}

	goFile := "package mdi\nvar iconMap = map[string]string" + string(jsonRaw)
	goFile = strings.Replace(goFile, "\"\n}", "\",\n}", 1)
	content, _ := format.Source([]byte(goFile))

	err = ioutil.WriteFile(gofile, content, os.ModePerm)
	if err != nil {
		fmt.Println("保存文件出错", err)
	} else {
		fmt.Println("保存 Weather 文件完毕", dest)
	}
	os.Remove(dest)
}
