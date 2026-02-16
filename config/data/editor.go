package data

import (
	"log"

	"github.com/jszwec/csvutil"

	"github.com/soulteary/flare/config/model"
)

// TODO Removed after private link feature support
type _BOOKMARK_REMOVE_PRIVATE struct {
	Name     string `yaml:"name"`
	URL      string `yaml:"link"`
	Icon     string `yaml:"icon,omitempty"`
	Desc     string `yaml:"desc,omitempty"`
	Category string `yaml:"category,omitempty"`
}

func removePrivateProp(input []model.Bookmark) (result []_BOOKMARK_REMOVE_PRIVATE) {
	for _, src := range input {
		var dest _BOOKMARK_REMOVE_PRIVATE
		dest.Name = src.Name
		dest.URL = src.URL
		dest.Icon = src.Icon
		dest.Desc = src.Desc
		dest.Category = src.Category
		result = append(result, dest)
	}
	return result
}

func restorePrivateProp(input []_BOOKMARK_REMOVE_PRIVATE) (result []model.Bookmark) {
	for _, src := range input {
		var dest model.Bookmark
		dest.Name = src.Name
		dest.URL = src.URL
		dest.Icon = src.Icon
		dest.Desc = src.Desc
		dest.Category = src.Category
		dest.Private = false
		result = append(result, dest)
	}
	return result
}

func GetBookmarksForEditor() (categories string, bookmarks string) {
	favoriteBookmarks, errFav := LoadFavoriteBookmarks()
	if errFav != nil {
		return "", ""
	}
	normalBookmarks, errNorm := LoadNormalBookmarks()
	if errNorm != nil {
		return "", ""
	}

	var mixedBookmarks []model.Bookmark

	var appendFixedCategoryForFavorite []model.Bookmark
	for _, item := range favoriteBookmarks.Items {
		// TODO Defined as a constant, provided for front-end use
		item.Category = "_FLARE_FIXED_CATEGORY"
		appendFixedCategoryForFavorite = append(appendFixedCategoryForFavorite, item)
	}

	mixedBookmarks = append(mixedBookmarks, appendFixedCategoryForFavorite...)
	mixedBookmarks = append(mixedBookmarks, normalBookmarks.Items...)

	categories = jsonStringify(normalBookmarks.Categories)
	bookmarks = jsonStringify(removePrivateProp(mixedBookmarks))

	return categories, bookmarks
}

func getCategoriesFromCSV(input string) (result []model.Category, err error) {
	var fixHead = []byte("ID,Name\n" + input)
	var decode []model.Category
	if err := csvutil.Unmarshal(fixHead, &decode); err != nil {
		return result, err
	}

	var validItem []model.Category

	for _, item := range decode {
		if item.Name != "" && item.ID != "" {
			validItem = append(validItem, item)
		}
	}
	return validItem, nil
}

func getBookmarksFromCSV(input string, categories []model.Category) (favoriteBookmarks []model.Bookmark, normalBookmarks []model.Bookmark, err error) {
	var fixHead = []byte("ID,Name,URL,Category,Icon,Desc\n" + input)
	var decode []_BOOKMARK_REMOVE_PRIVATE

	if err := csvutil.Unmarshal(fixHead, &decode); err != nil {
		return favoriteBookmarks, normalBookmarks, err
	}

	bookmarks := restorePrivateProp(decode)
	for _, bookmark := range bookmarks {
		if bookmark.Name != "" && bookmark.URL != "" {
			// TODO Defined as a constant, provided for front-end use
			if bookmark.Category == "[Flare 应用]" || bookmark.Category == "" {
				bookmark.Category = ""
				favoriteBookmarks = append(favoriteBookmarks, bookmark)
			} else {
				for _, category := range categories {
					if category.Name == bookmark.Category {
						bookmark.Category = category.ID
						break
					}
				}
				normalBookmarks = append(normalBookmarks, bookmark)
			}
		}
	}

	return favoriteBookmarks, normalBookmarks, nil
}

func UpdateBookmarksFromEditor(categoriesCSV string, bookmakrsCSV string) bool {

	categories, err := getCategoriesFromCSV(categoriesCSV)
	if err != nil {
		log.Println("提交数据解析出现问题，请检查分类数据格式", err)
		return false
	}

	favorite, normal, err := getBookmarksFromCSV(bookmakrsCSV, categories)
	if err != nil {
		log.Println("提交数据解析出现问题，请检查书签数据格式", err)
		return false
	}

	var normalBookmarks model.Bookmarks
	normalBookmarks.Items = normal
	normalBookmarks.Categories = categories
	SaveNormalBookmarks(normalBookmarks)

	var favoriteBookmarks model.Bookmarks
	favoriteBookmarks.Items = favorite
	SaveFavoriteBookmarks(favoriteBookmarks)

	return true
}
