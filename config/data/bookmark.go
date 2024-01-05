package FlareData

import (
	"fmt"
	"log"
	"strconv"

	"gopkg.in/yaml.v2"

	FlareModel "github.com/soulteary/flare/config/model"
)

func initBookmarks(filePath string, isFavorite bool) (result FlareModel.Bookmarks, err error) {

	const exampleName = "示例链接"
	const exmapleLink = "https://link.example.com"
	const exampleDesc = "链接描述文本"

	var exampleIcons = [28]string{
		"evernote", "FireHydrant", "email", "MicrosoftOnenote",
		"Robber", "EvPlugType1", "FileImage", "WeatherHazy",
		"checkDecagram", "sofaOutline", "foodCroissant", "musicCircleOutline", "eraser",
		"BowArrow", "KeyboardOutline", "Incognito", "mastodon", "messageCog",
		"alphaFCircleOutline", "alphaLCircleOutline", "alphaACircleOutline", "alphaRCircleOutline", "alphaECircleOutline",
		"accountSupervisorCircle", "flask", "cityVariantOutline", "alphaYCircleOutline", "sproutOutline",
	}

	if isFavorite {
		// with desc
		for i := 0; i < 4; i++ {
			var bookmark FlareModel.Bookmark
			bookmark.Name = exampleName
			bookmark.URL = exmapleLink
			bookmark.Icon = exampleIcons[i]
			bookmark.Desc = exampleDesc
			result.Items = append(result.Items, bookmark)
		}
		// without desc
		for i := 0; i < 4; i++ {
			var bookmark FlareModel.Bookmark
			bookmark.Name = exampleName
			bookmark.URL = exmapleLink
			bookmark.Icon = exampleIcons[i+4]
			result.Items = append(result.Items, bookmark)
		}
	} else {
		var categories []FlareModel.Category
		var bookmarks []FlareModel.Bookmark
		const prefix = "cate-id-"

		for i := 0; i < 4; i++ {
			var category FlareModel.Category
			category.Name = "链接分类" + strconv.Itoa(i+1)
			category.ID = prefix + strconv.Itoa(i)
			categories = append(categories, category)
		}
		result.Categories = categories

		for i := 0; i < 20; i++ {
			var bookmark FlareModel.Bookmark
			bookmark.Name = exampleName
			bookmark.URL = exmapleLink
			bookmark.Icon = exampleIcons[8+i]
			bookmark.Category = prefix + strconv.Itoa(i%4)
			bookmarks = append(bookmarks, bookmark)
		}
		result.Items = bookmarks
	}

	out, err := yaml.Marshal(result)
	if err != nil {
		log.Println("初始化程序时出错。")
		return result, err
	}

	ok := saveFile(filePath, out)
	if !ok {
		log.Println("保存初始配置失败。")
		return result, err
	}

	return result, nil
}

func saveBookmarksToYamlFile(name string, data FlareModel.Bookmarks) (bool, error) {
	out, err := yaml.Marshal(data)
	if err != nil {
		log.Println("转换数据格式失败", name)
		return false, err
	}

	filePath := getConfigPath(name)
	ok := saveFile(filePath, out)
	if !ok {
		log.Println("保存数据为书签失败")
		return false, err
	}

	return true, nil
}

func loadBookmarksFromYamlFile(name string, isFavorite bool) (result FlareModel.Bookmarks) {
	filePath := getConfigPath(name)

	if checkExists(filePath) {
		configFile := readFile(filePath, true)
		parseErr := yaml.Unmarshal(configFile, &result)
		if parseErr != nil {
			log.Fatalf("解析配置文件" + name + "错误，请检查配置文件内容。")
		}
	} else {
		fmt.Println("找不到配置文件" + name + "，创建默认配置。")
		var createErr error
		result, createErr = initBookmarks(filePath, isFavorite)
		if createErr != nil {
			log.Fatalf("尝试创建应用配置文件" + name + "失败")
		}
	}

	return result
}

func SaveFavoriteBookmarks(data FlareModel.Bookmarks) bool {
	result, _ := saveBookmarksToYamlFile("apps", data)
	return result
}

func SaveNormalBookmarks(data FlareModel.Bookmarks) bool {
	result, _ := saveBookmarksToYamlFile("bookmarks", data)
	return result
}

func LoadFavoriteBookmarks() (result FlareModel.Bookmarks) {
	return loadBookmarksFromYamlFile("apps", true)
}

func LoadNormalBookmarks() (result FlareModel.Bookmarks) {
	return loadBookmarksFromYamlFile("bookmarks", false)
}
