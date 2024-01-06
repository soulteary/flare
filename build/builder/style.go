package builder

import (
	"bufio"
	"bytes"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

func TaskForStyles(gofile string) {

	content := doMinifyCSS([]string{
		"embed/assets/css/base.css",
		// home
		"embed/assets/css/home/apps.css",
		"embed/assets/css/home/bookmarks.css",
		"embed/assets/css/home/hero.css",
		"embed/assets/css/home/search.css",
		"embed/assets/css/home/toolbar.css",
		// settings
		"embed/assets/css/settings/layout.css",
		"embed/assets/css/settings/sidebar.css",
		"embed/assets/css/settings/theme.css",
	})

	initInlineStyle("/** 月出惊山鸟，时鸣春涧中。**/ "+content, gofile)

	fmt.Println("打包样式文件 ... [OK]")

}

func initInlineStyle(data string, gofile string) {
	content := "package FlareDefine\nconst PAGE_INLINE_STYLE = " + `"` + strings.Replace(data, "\"", "\\\"", -1) + `"`
	fmtContent, err := format.Source([]byte(content))
	if err != nil {
		fmt.Println("序列化内容失败", err)
	}
	err = os.WriteFile(gofile, fmtContent, os.ModePerm)
	if err != nil {
		fmt.Println("保存文件出错", err)
	}
}

func doMinifyCSS(cssPathes []string) string {
	cssAll := concatenateCSS(cssPathes)
	cssAllNoComments := RemoveCppStyleComments(RemoveCStyleComments(cssAll))

	// read line by line
	minifiedCss := ""
	scanner := bufio.NewScanner(bytes.NewReader(cssAllNoComments))
	for scanner.Scan() {
		// all leading and trailing white space of each line are removed
		minifiedCss += strings.TrimSpace(scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		panic(err)
	}

	return minifiedCss
}

// https://siongui.github.io/2016/03/09/go-minify-css/

func RemoveCStyleComments(content []byte) []byte {
	// http://blog.ostermiller.org/find-comment
	ccmt := regexp.MustCompile(`/\*([^*]|[\r\n]|(\*+([^*/]|[\r\n])))*\*+/`)
	return ccmt.ReplaceAll(content, []byte(""))
}

func RemoveCppStyleComments(content []byte) []byte {
	cppcmt := regexp.MustCompile(`//.*`)
	return cppcmt.ReplaceAll(content, []byte(""))
}

func concatenateCSS(cssPathes []string) []byte {
	var cssAll []byte
	for _, cssPath := range cssPathes {
		println("concatenating " + cssPath + " ...")
		b, err := os.ReadFile(filepath.Clean(cssPath))
		if err != nil {
			panic(err)
		}
		cssAll = append(cssAll, b...)
	}
	return cssAll
}
