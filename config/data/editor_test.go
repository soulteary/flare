package FlareData

import (
	"testing"

	FlareModel "github.com/soulteary/flare/config/model"
)

func TestGetBookmarksDataAsJSON(t *testing.T) {
	categories, bookmarks := GetBookmarksForEditor()
	if len(categories) == 0 || len(bookmarks) == 0 {
		t.Fatal("GetBookmarksForEditor Failed")
	}
}

func TestGetAndUpdateBookmarksFromEditor(t *testing.T) {

	const categories = `1,链接分类1
2,链接分类2
3,链接分类3
4,链接分类4`
	const bookmarks = `1,示例链接,https://link.example.com,[Flare 应用],evernote,链接描述文本
2,示例链接,https://link.example.com,[Flare 应用],FireHydrant,链接描述文本
3,示例链接,https://link.example.com,[Flare 应用],email,链接描述文本
4,示例链接,https://link.example.com,[Flare 应用],MicrosoftOnenote,链接描述文本
5,示例链接,https://link.example.com,[Flare 应用],Robber,
6,示例链接,https://link.example.com,[Flare 应用],EvPlugType1,
7,示例链接,https://link.example.com,[Flare 应用],FileImage,
8,示例链接,https://link.example.com,[Flare 应用],WeatherHazy,
9,示例链接,https://link.example.com,链接分类1,checkDecagram,
10,示例链接,https://link.example.com,链接分类1,eraser,
11,示例链接,https://link.example.com,链接分类1,mastodon,
12,示例链接,https://link.example.com,链接分类1,alphaACircleOutline,
13,示例链接,https://link.example.com,链接分类1,flask,
14,示例链接,https://link.example.com,链接分类2,sofaOutline,
15,示例链接,https://link.example.com,链接分类2,BowArrow,
16,示例链接,https://link.example.com,链接分类2,messageCog,
17,示例链接,https://link.example.com,链接分类2,alphaRCircleOutline,
18,示例链接,https://link.example.com,链接分类2,cityVariantOutline,
19,示例链接,https://link.example.com,链接分类3,foodCroissant,
20,示例链接,https://link.example.com,链接分类3,KeyboardOutline,
21,示例链接,https://link.example.com,链接分类3,alphaFCircleOutline,
22,示例链接,https://link.example.com,链接分类3,alphaECircleOutline,
23,示例链接,https://link.example.com,链接分类3,alphaYCircleOutline,
24,示例链接,https://link.example.com,链接分类4,musicCircleOutline,
25,示例链接,https://link.example.com,链接分类4,Incognito,
26,示例链接,https://link.example.com,链接分类4,alphaLCircleOutline,
27,示例链接,https://link.example.com,链接分类4,accountSupervisorCircle,
28,示例链接,https://link.example.com,链接分类4,sproutOutline,`

	updated := UpdateBookmarksFromEditor(categories, bookmarks)
	if !updated {
		t.Fatal("UpdateBookmarksFromEditor Failed")
	}

	bookmarkCategories, ok := getCategoriesFromCSV(categories)
	if ok != nil {
		t.Fatal("getCategoriesFromCSV Failed")
	}

	_, _, ok = getBookmarksFromCSV(bookmarks, bookmarkCategories)
	if ok != nil {
		t.Fatal("getBookmarksFromCSV Failed")
	}
}

func TestPropsRemoveAndRestore(t *testing.T) {
	var input []FlareModel.Bookmark
	input = append(input, FlareModel.Bookmark{Private: true})

	removed := restorePrivateProp(removePrivateProp(input))
	for i := 0; i < len(removed); i++ {
		if removed[i].Private != false {
			t.Fatal("Remove and restore private prop Failed")
		}
	}
}
