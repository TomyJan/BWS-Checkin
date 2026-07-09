package qrcode

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"errors"
	"net/url"
	"strings"

	goqrcode "github.com/skip2/go-qrcode"
)

const (
	bwsLivePage = "https://www.bilibili.com/blackboard/era/bws2026-live.html"
	bwsAESKey   = "f2CmYe*nls&MW*75"
	bwsAESIV    = "7VKmLf4NvGO#83Y@"
)

func BWSURL(mid string) (string, error) {
	mid = strings.TrimSpace(mid)
	if mid == "" {
		return "", errors.New("mid is required")
	}
	encrypted, err := encryptBWSMID(mid)
	if err != nil {
		return "", err
	}
	return bwsLivePage + "?key=" + url.QueryEscape(encrypted) + "#/map", nil
}

func BWSPNG(mid string) ([]byte, error) {
	qrURL, err := BWSURL(mid)
	if err != nil {
		return nil, err
	}
	return goqrcode.Encode(qrURL, goqrcode.Medium, 1024)
}

func PNGDataURL(value string, size int) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", errors.New("qrcode value is required")
	}
	if size <= 0 {
		size = 256
	}
	png, err := goqrcode.Encode(value, goqrcode.Medium, size)
	if err != nil {
		return "", err
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(png), nil
}

func encryptBWSMID(mid string) (string, error) {
	block, err := aes.NewCipher([]byte(bwsAESKey))
	if err != nil {
		return "", err
	}
	plain := pkcs7Pad([]byte(mid), block.BlockSize())
	encrypted := make([]byte, len(plain))
	cipher.NewCBCEncrypter(block, []byte(bwsAESIV)).CryptBlocks(encrypted, plain)
	return base64.StdEncoding.EncodeToString(encrypted), nil
}

func pkcs7Pad(input []byte, blockSize int) []byte {
	padding := blockSize - len(input)%blockSize
	output := make([]byte, len(input)+padding)
	copy(output, input)
	for i := len(input); i < len(output); i++ {
		output[i] = byte(padding)
	}
	return output
}
