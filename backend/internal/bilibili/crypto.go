package bilibili

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"
)

type cookieRecord struct {
	Name     string    `json:"name"`
	Value    string    `json:"value"`
	Domain   string    `json:"domain"`
	Path     string    `json:"path"`
	Expires  time.Time `json:"expires"`
	Secure   bool      `json:"secure"`
	HTTPOnly bool      `json:"httpOnly"`
	SameSite int       `json:"sameSite"`
}

func EncryptCookieJar(secret string, cookies []*http.Cookie) (string, error) {
	if secret == "" {
		return "", errors.New("cookie secret is required")
	}
	plain, err := json.Marshal(cookieRecords(cookies))
	if err != nil {
		return "", err
	}
	gcm, err := newGCM(secret)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := gcm.Seal(nonce, nonce, plain, nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func DecryptCookieJar(secret string, encrypted string) ([]*http.Cookie, error) {
	if secret == "" {
		return nil, errors.New("cookie secret is required")
	}
	raw, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		return nil, err
	}
	gcm, err := newGCM(secret)
	if err != nil {
		return nil, err
	}
	if len(raw) < gcm.NonceSize() {
		return nil, errors.New("cookie ciphertext is too short")
	}
	nonce := raw[:gcm.NonceSize()]
	ciphertext := raw[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}
	var records []cookieRecord
	if err := json.Unmarshal(plain, &records); err != nil {
		return nil, err
	}
	return httpCookies(records), nil
}

func newGCM(secret string) (cipher.AEAD, error) {
	key := sha256.Sum256([]byte(secret))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func cookieRecords(cookies []*http.Cookie) []cookieRecord {
	records := make([]cookieRecord, 0, len(cookies))
	for _, cookie := range cookies {
		if cookie == nil {
			continue
		}
		records = append(records, cookieRecord{
			Name:     cookie.Name,
			Value:    cookie.Value,
			Domain:   cookie.Domain,
			Path:     cookie.Path,
			Expires:  cookie.Expires,
			Secure:   cookie.Secure,
			HTTPOnly: cookie.HttpOnly,
			SameSite: int(cookie.SameSite),
		})
	}
	return records
}

func httpCookies(records []cookieRecord) []*http.Cookie {
	cookies := make([]*http.Cookie, 0, len(records))
	for _, record := range records {
		cookies = append(cookies, &http.Cookie{
			Name:     record.Name,
			Value:    record.Value,
			Domain:   record.Domain,
			Path:     record.Path,
			Expires:  record.Expires,
			Secure:   record.Secure,
			HttpOnly: record.HTTPOnly,
			SameSite: http.SameSite(record.SameSite),
		})
	}
	return cookies
}
