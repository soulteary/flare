package FlareFn

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"
)

var client = &http.Client{Timeout: 3 * time.Second}

func GetJSON(url string, target interface{}) error {
	r, err := client.Get(url)
	if err != nil {
		return err
	}
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(target)
}

func GetHTML(url string) (result string, err error) {
	r, err := client.Get(url)
	if err != nil {
		return "", err
	}

	defer r.Body.Close()

	if r.StatusCode != http.StatusOK {
		return "", errors.New(strconv.Itoa(r.StatusCode))
	}

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		return "", err
	}
	return string(bodyBytes), nil
}
