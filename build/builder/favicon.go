package builder

import "fmt"

func TaskForFavicon(src string, dest string) {
	_Copy(src, dest)
	fmt.Println("复制静态资源 ... [OK]")
}
