package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/internal/logger"
)

// cookieSecretFileName lives beside config.yml in the application's working directory.
const cookieSecretFileName = ".flare-cookie-secret"

// initializeCookieSecret retains safe explicit configuration and migrates unsafe keys without downtime.
// Generated keys contain 32 random bytes encoded as hexadecimal, and persisted keys survive restarts.
func initializeCookieSecret(configured string) (string, error) {
	trimmed := strings.TrimSpace(configured)
	if trimmed != define.DEFAULT_COOKIE_SECRET && len(trimmed) >= minimumCookieSecretLength {
		return configured, nil
	}

	reason, explanation := "short", "不足 32 字节"
	switch trimmed {
	case "":
		reason, explanation = "empty", "未配置"
	case define.DEFAULT_COOKIE_SECRET:
		reason, explanation = "default", "使用默认值"
	}

	secret, created, err := loadOrCreateCookieSecret(cookieSecretFileName)
	if err != nil {
		// Filesystem failures must not restore the old unsafe key or prevent an upgrade.
		// Do not log the underlying error: paths or damaged contents may contain secrets.
		fallback, randomErr := generateCookieSecret(rand.Reader)
		if randomErr != nil {
			return "", randomErr
		}
		logger.GetLogger().Warn("Cookie 密钥"+explanation+"；程序已自动生成具有 32 字节随机熵的安全长密钥，但读取、校验或保存失败，密钥未持久化；已有登录状态失效，请重新登录，重启后需要重新登录",
			"reason", reason, "action", "generated", "persisted", false)
		return fallback, nil
	}
	if created {
		logger.GetLogger().Warn("Cookie 密钥"+explanation+"；程序已自动生成并保存具有 32 字节随机熵的安全长密钥，已有登录状态失效，请重新登录",
			"reason", reason, "action", "generated", "persisted", true)
	} else {
		logger.GetLogger().Warn("Cookie 密钥"+explanation+"；程序已复用已保存的安全长密钥",
			"reason", reason, "action", "reused", "persisted", true)
	}
	return secret, nil
}

// generateCookieSecret reads exactly 32 bytes from a secure source; an argument avoids global test hooks.
func generateCookieSecret(source io.Reader) (string, error) {
	var raw [minimumCookieSecretLength]byte
	if _, err := io.ReadFull(source, raw[:]); err != nil {
		return "", fmt.Errorf("无法生成安全 Cookie 密钥: %w", err)
	}
	return hex.EncodeToString(raw[:]), nil
}

// readCookieSecret only accepts a regular file holding one 64-character hexadecimal key.
// Checking file identity before reading prevents a swapped symlink from redirecting permission changes.
func readCookieSecret(path string) (string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", errors.New("密钥文件不是常规文件")
	}
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	openedInfo, err := file.Stat()
	if err != nil {
		return "", err
	}
	if !openedInfo.Mode().IsRegular() || !os.SameFile(info, openedInfo) {
		return "", errors.New("密钥文件在读取前发生变化")
	}

	// Limit the read so damaged or unexpectedly large files cannot exhaust memory.
	content, err := io.ReadAll(io.LimitReader(file, minimumCookieSecretLength*2+1))
	if err != nil {
		return "", err
	}
	if len(content) != minimumCookieSecretLength*2 {
		return "", errors.New("密钥文件格式无效")
	}
	if _, err := hex.DecodeString(string(content)); err != nil {
		return "", errors.New("密钥文件格式无效")
	}
	if openedInfo.Mode().Perm() != 0600 || openedInfo.Mode()&(os.ModeSetuid|os.ModeSetgid|os.ModeSticky) != 0 {
		if err := file.Chmod(0600); err != nil {
			return "", err
		}
	}
	return string(content), nil
}

// loadOrCreateCookieSecret publishes a fully written key without replacing an existing file.
// Linking a temporary file makes concurrent first starts agree on one winner, then all re-read it.
func loadOrCreateCookieSecret(path string) (string, bool, error) {
	secret, err := readCookieSecret(path)
	if err == nil {
		return secret, false, nil
	}
	if !os.IsNotExist(err) {
		return "", false, err
	}
	generated, err := generateCookieSecret(rand.Reader)
	if err != nil {
		return "", false, err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+"-*")
	if err != nil {
		return "", false, err
	}
	defer os.Remove(temporary.Name())
	defer temporary.Close()
	if err = temporary.Chmod(0600); err != nil {
		return "", false, err
	}
	if _, err = temporary.WriteString(generated); err != nil {
		return "", false, err
	}
	if err = temporary.Sync(); err != nil {
		return "", false, err
	}
	if err = temporary.Close(); err != nil {
		return "", false, err
	}
	created := true
	if err = os.Link(temporary.Name(), path); err != nil {
		if !os.IsExist(err) {
			return "", false, err
		}
		created = false
	}
	if created {
		// Flush the directory entry as well as the file before claiming persistence.
		directory, openErr := os.Open(filepath.Dir(path))
		if openErr != nil {
			return "", false, openErr
		}
		syncErr := directory.Sync()
		closeErr := directory.Close()
		if syncErr != nil {
			return "", false, syncErr
		}
		if closeErr != nil {
			return "", false, closeErr
		}
	}
	secret, err = readCookieSecret(path)
	if err != nil {
		return "", false, err
	}
	return secret, created, nil
}
