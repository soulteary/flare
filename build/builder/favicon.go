package builder

import (
	"fmt"
	"log"
)

func TaskForFavicon(src string, dest string) {
	if err := _Copy(src, dest); err != nil {
		log.Fatal(err)
	}
	fmt.Println("复制静态资源 ... [OK]")
}
