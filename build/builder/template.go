package builder

import (
	"fmt"
	"io/ioutil"
	"os"

	Minify "github.com/tdewolff/minify/v2"
	MinifyCSS "github.com/tdewolff/minify/v2/css"
	MinifyHTML "github.com/tdewolff/minify/v2/html"
	MinifySVG "github.com/tdewolff/minify/v2/svg"
)

func TaskForTemplates() {
	os.RemoveAll("pkg/templates/html")
	_PrepareDirectory("pkg/templates/html")
	_CopyDirectory("embed/templates", "pkg/templates/html")
	fmt.Println("复制模版文件 ... [OK]")

	minifyFilesByPathAndType("pkg/templates/html", "*.html", "text/html")
	os.RemoveAll("tmp")
}

func minifyFilesByPathAndType(filePath string, fileFilter string, mimeType string) {
	m := Minify.New()
	m.AddFunc("text/html", MinifyHTML.Minify)
	m.Add("text/html", &MinifyHTML.Minifier{
		KeepDocumentTags: true,
		KeepQuotes:       false,
	})

	m.AddFunc("image/svg+xml", MinifySVG.Minify)
	m.AddFunc("text/css", MinifyCSS.Minify)

	files, _ := _WalkMatch(filePath, fileFilter)

	for _, file := range files {
		fileRaw, err := ioutil.ReadFile(file)
		if err != nil {
			fmt.Println("读取文件出错", file)
		} else {
			fileMinified, err := m.Bytes(mimeType, fileRaw)
			if err != nil {
				fmt.Println("压缩文件出错", file, err)
			} else {
				err = os.WriteFile(file, fileMinified, os.ModePerm)
				if err != nil {
					fmt.Println("保存文件出错", file, err)
				} else {
					fmt.Println("压缩文件完毕", file)
				}
			}
		}
	}
}
