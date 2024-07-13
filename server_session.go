package gwv

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type Cryptor struct {
	SecretKey      []byte
	CookieName     string
	MakeCookieFunc MakeCookieFunc
}

func NewSimpleCryptor(secretKey []byte, cookieName string) *Cryptor {
	return &Cryptor{
		SecretKey:  secretKey,
		CookieName: cookieName,
		MakeCookieFunc: MakeCookieFunc(func(w http.ResponseWriter, r *http.Request) *http.Cookie {
			return &http.Cookie{
				Name:     cookieName,
				Path:     "/",
				MaxAge:   360000,
				HttpOnly: false,
			}
		}),
	}
}

type MakeCookieFunc func(w http.ResponseWriter, r *http.Request) *http.Cookie

func (sc *Cryptor) Write(v interface{}, w http.ResponseWriter, r *http.Request) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}

	iv := make([]byte, aes.BlockSize)
	if _, err := rand.Read(iv); err != nil {
		return err
	}

	block, err := aes.NewCipher(sc.SecretKey)
	if err != nil {
		return err
	}

	cfb := cipher.NewCFBEncrypter(block, iv)
	ciphertext := make([]byte, len(b))
	cfb.XORKeyStream(ciphertext, b)

	cookie := sc.MakeCookieFunc(w, r)
	cookie.Value = base64.RawURLEncoding.EncodeToString(iv) + "," + base64.RawURLEncoding.EncodeToString(ciphertext)
	http.SetCookie(w, cookie)

	return nil
}

func (sc *Cryptor) Read(v interface{}, r *http.Request) error {
	c, err := r.Cookie(sc.CookieName)
	if err != nil {
		return err
	}

	parts := strings.Split(c.Value, ",")
	if len(parts) != 2 {
		return fmt.Errorf("invalid cookie value")
	}

	iv, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return err
	}

	ciphertext, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return err
	}

	block, err := aes.NewCipher(sc.SecretKey)
	if err != nil {
		return err
	}

	cfb := cipher.NewCFBDecrypter(block, iv)
	plaintext := make([]byte, len(ciphertext))
	cfb.XORKeyStream(plaintext, ciphertext)

	if err := json.Unmarshal(plaintext, v); err != nil {
		return err
	}

	return nil
}

func (sc *Cryptor) Clear(w http.ResponseWriter, r *http.Request) {
	c := sc.MakeCookieFunc(w, r)
	c.MaxAge = -1
	http.SetCookie(w, c)
}
