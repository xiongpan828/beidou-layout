// Package signaturex URL 签名：hex(HMAC-SHA256(secret, method + ":" + key + ":" + expire))。
//
// method 参与签名，上传（PUT）与下载（GET）的签名不能互用；expire 是 Unix 秒，过期由调用方判断。
package signaturex

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
)

// Sign 生成签名（hex）
func Sign(secret, method, key string, expireAt int64) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(method + ":" + key + ":" + strconv.FormatInt(expireAt, 10)))
	return hex.EncodeToString(mac.Sum(nil))
}

// Valid 常量时间比对签名（不判断过期）
func Valid(secret, method, key string, expireAt int64, sig string) bool {
	got, err := hex.DecodeString(sig)
	if err != nil {
		return false
	}
	want, _ := hex.DecodeString(Sign(secret, method, key, expireAt))
	return hmac.Equal(got, want)
}
