package qrcode_test

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"net/url"
	"strings"
	"testing"

	"bws-checkin/backend/internal/qrcode"
)

func TestBWSURLIsStableAndDecryptable(t *testing.T) {
	got, err := qrcode.BWSURL("123456")
	if err != nil {
		t.Fatalf("build bws url: %v", err)
	}
	if !strings.HasPrefix(got, "https://www.bilibili.com/blackboard/era/bws2026-live.html?key=") {
		t.Fatalf("url = %q, want BWS live page with key query", got)
	}

	again, err := qrcode.BWSURL("123456")
	if err != nil {
		t.Fatalf("build bws url again: %v", err)
	}
	if got != again {
		t.Fatalf("url is not stable: %q != %q", got, again)
	}

	parsed, err := url.Parse(got)
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}
	if decrypted := decryptBWSKey(t, parsed.Query().Get("key")); decrypted != "123456" {
		t.Fatalf("decrypted key = %q, want 123456", decrypted)
	}
}

func TestBWSPNG(t *testing.T) {
	png, err := qrcode.BWSPNG("123456")
	if err != nil {
		t.Fatalf("build bws png: %v", err)
	}
	if !bytes.HasPrefix(png, []byte{0x89, 'P', 'N', 'G'}) {
		t.Fatalf("png has invalid header: %x", png[:4])
	}
}

func decryptBWSKey(t *testing.T, encrypted string) string {
	t.Helper()
	ciphertext, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		t.Fatalf("decode key: %v", err)
	}
	block, err := aes.NewCipher([]byte("f2CmYe*nls&MW*75"))
	if err != nil {
		t.Fatalf("new aes cipher: %v", err)
	}
	if len(ciphertext)%block.BlockSize() != 0 {
		t.Fatalf("ciphertext length %d is not block aligned", len(ciphertext))
	}
	plain := make([]byte, len(ciphertext))
	cipher.NewCBCDecrypter(block, []byte("7VKmLf4NvGO#83Y@")).CryptBlocks(plain, ciphertext)
	padding := int(plain[len(plain)-1])
	if padding == 0 || padding > block.BlockSize() || padding > len(plain) {
		t.Fatalf("invalid pkcs7 padding length %d", padding)
	}
	for _, b := range plain[len(plain)-padding:] {
		if int(b) != padding {
			t.Fatalf("invalid pkcs7 padding byte %d, want %d", b, padding)
		}
	}
	return string(plain[:len(plain)-padding])
}
