package data

import (
	"bytes"
	"crypto/md5" //#nosec
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"strings"
	"time"
)

func jsonStringify(data interface{}) string {
	buff := bytes.NewBuffer([]byte{})
	encoder := json.NewEncoder(buff)
	encoder.SetEscapeHTML(false)
	encoder.Encode(data)
	return strings.TrimSpace(buff.String())
}

func MaskTextWithStars(input string) string {
	chars := []rune(input)
	mask := chars
	count := len(chars)
	for i := 0; i < count; i++ {
		if i != 0 && i != count-1 {
			mask[i] = '*'
		}
	}
	return string(mask)
}

func GenerateRandomString(size int) string {
	id := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, id); err != nil {
		data := []byte(time.Now().String())
		hash := md5.Sum(data)
		return hex.EncodeToString(hash[0:size])
	}
	return hex.EncodeToString(id)[:size]
}

func Base64Encode(input string) string {
	return base64.StdEncoding.EncodeToString([]byte(input))
}
