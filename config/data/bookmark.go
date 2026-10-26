package data

import (
	"fmt"
	"log"
	"strconv"

	"go.yaml.in/yaml/v3"

	"github.com/soulteary/flare/config/model"
)

func initBookmarks(filePath string, isFavorite bool) (result model.Bookmarks, err error) {
	recoveries, recoveryErr := EditorRecoveryDirectories()
	if recoveryErr != nil {
		return result, recoveryErr
	}
	if len(recoveries) > 0 {
		return result, fmt.Errorf("live bookmark file %s is missing while editor recovery copies exist; inspect %s before restoring files", filePath, recoveries[0])
	}

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
			var bookmark model.Bookmark
			bookmark.Name = exampleName
			bookmark.URL = exmapleLink
			bookmark.Icon = exampleIcons[i]
			bookmark.Desc = exampleDesc
			result.Items = append(result.Items, bookmark)
		}
		// without desc
		for i := 0; i < 4; i++ {
			var bookmark model.Bookmark
			bookmark.Name = exampleName
			bookmark.URL = exmapleLink
			bookmark.Icon = exampleIcons[i+4]
			result.Items = append(result.Items, bookmark)
		}
	} else {
		var categories []model.Category
		var bookmarks []model.Bookmark
		const prefix = "cate-id-"

		for i := 0; i < 4; i++ {
			var category model.Category
			category.Name = "链接分类" + strconv.Itoa(i+1)
			category.ID = prefix + strconv.Itoa(i)
			categories = append(categories, category)
		}
		result.Categories = categories

		for i := 0; i < 20; i++ {
			var bookmark model.Bookmark
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
		return result, fmt.Errorf("保存初始配置失败: %s", filePath)
	}

	return result, nil
}

func saveBookmarksToYamlFile(name string, data model.Bookmarks) (bool, error) {
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
	invalidateFileCache(name)
	return true, nil
}

func loadBookmarksFromYamlFile(name string, isFavorite bool) (model.Bookmarks, error) {
	snapshot, err := loadBookmarkSnapshot(name, isFavorite)
	if err != nil {
		return model.Bookmarks{}, err
	}
	return cloneBookmarks(snapshot.value), nil
}

// Caller holds bookmarkMu. Returned snapshots remain private and immutable.
func loadBookmarkSnapshot(name string, isFavorite bool) (*parsedBookmarks, error) {
	if !checkExists(name + ".yml") {
		fmt.Println("找不到配置文件" + name + "，创建默认配置。")
		invalidateFileCache(name)
		if _, err := initBookmarks(getConfigPath(name), isFavorite); err != nil {
			return nil, fmt.Errorf("尝试创建应用配置文件 %s 失败: %w", name, err)
		}
	}
	return readExistingBookmarkSnapshot(name)
}

func readExistingBookmarkSnapshot(name string) (*parsedBookmarks, error) {
	raw, err := readFileCached(name, func() ([]byte, error) { return readFile(getConfigPath(name)) })
	if err != nil {
		return nil, fmt.Errorf("读取配置文件 %s: %w", name, err)
	}
	snapshot := parsedBookmarkData(name, raw)
	if snapshot.err != nil {
		return nil, fmt.Errorf("解析配置文件 %s 错误，请检查配置文件内容: %w", name, snapshot.err)
	}
	return snapshot, nil
}

func navigationSnapshot(name string) (*parsedBookmarks, error) {
	bookmarkMu.RLock()
	if checkExists(name + ".yml") {
		snapshot, err := readExistingBookmarkSnapshot(name)
		bookmarkMu.RUnlock()
		return snapshot, err
	}
	bookmarkMu.RUnlock()
	// Initialization writes to disk and must exclude other readers and writers.
	bookmarkMu.Lock()
	defer bookmarkMu.Unlock()
	return loadBookmarkSnapshot(name, name == "apps")
}

func SaveFavoriteBookmarks(data model.Bookmarks) bool {
	bookmarkMu.Lock()
	defer bookmarkMu.Unlock()
	result, err := saveBookmarksToYamlFile("apps", data)
	return err == nil && result
}

func SaveNormalBookmarks(data model.Bookmarks) bool {
	bookmarkMu.Lock()
	defer bookmarkMu.Unlock()
	result, err := saveBookmarksToYamlFile("bookmarks", data)
	return err == nil && result
}

func LoadFavoriteBookmarks() (model.Bookmarks, error) {
	snapshot, err := navigationSnapshot("apps")
	if err != nil {
		return model.Bookmarks{}, err
	}
	return cloneBookmarks(snapshot.value), nil
}

func LoadNormalBookmarks() (model.Bookmarks, error) {
	snapshot, err := navigationSnapshot("bookmarks")
	if err != nil {
		return model.Bookmarks{}, err
	}
	return cloneBookmarks(snapshot.value), nil
}
