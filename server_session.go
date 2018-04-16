package gwv

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
)

type Cryptor struct {
	SecretKey      []byte
	CookieName     string
	MakeCookieFunc MakeCookieFunc
}

//NewSimpleCryptor returns a pointer to crypto cookie object
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

// makes an empty cookie, no value
type MakeCookieFunc func(w http.ResponseWriter, r *http.Request) *http.Cookie

//Write seralize and encrypts values and write them to a cookie
func (sc *Cryptor) Write(v interface{}, w http.ResponseWriter, r *http.Request) error {

	// marshall data
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}

	// make init vector
	iv := make([]byte, 16)
	_, err = rand.Read(iv)
	if err != nil {
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

//Read returns the decrypted value of the cookie
func (sc *Cryptor) Read(v interface{}, r *http.Request) error {

	c, err := r.Cookie(sc.CookieName)
	if err != nil {

		return err
	}

	cookieValueParts := strings.Split(c.Value, ",")

	// extract init vector
	iv, err := base64.RawURLEncoding.DecodeString(cookieValueParts[0])
	if err != nil {

		return err
	}

	// extract value
	b, err := base64.RawURLEncoding.DecodeString(cookieValueParts[1])
	if err != nil {

		return err
	}

	block, err := aes.NewCipher(sc.SecretKey)
	if err != nil {

		return err
	}

	cfb := cipher.NewCFBDecrypter(block, iv)
	plaintext := make([]byte, len(b))
	cfb.XORKeyStream(plaintext, b)

	err = json.Unmarshal(plaintext, v)
	if err != nil {

		return err
	}

	return nil
}

//Clear removes the cookie (effectively destroying the session)
func (sc *Cryptor) Clear(w http.ResponseWriter, r *http.Request) {
	c := sc.MakeCookieFunc(w, r)
	c.MaxAge = -1
	http.SetCookie(w, c)
}
