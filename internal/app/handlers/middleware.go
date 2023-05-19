package handlers

import (
	"compress/gzip"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

type Middleware func(http.Handler) http.Handler

func (c *Controller) MiddlewaresConveyor(h http.Handler) http.Handler {
	middlewares := []Middleware{gzipMiddleware, c.cookieMiddleware}
	for _, middleware := range middlewares {
		h = middleware(h)
	}
	return h
}

type gzipWriter struct {
	http.ResponseWriter
	Writer io.Writer
}

func (w gzipWriter) Write(b []byte) (int, error) {
	return w.Writer.Write(b)
}

func gzipMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.Header.Get("Content-Encoding"), "gzip") {
			gz, err := gzip.NewReader(r.Body)
			if err != nil {
				log.Print("gzipMiddleware: new reader err: ", err.Error())
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			defer func() {
				_ = gz.Close()
			}()

			r.Body = gz
		}

		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}

		gz, err := gzip.NewWriterLevel(w, gzip.BestSpeed)
		if err != nil {
			log.Print("gzipMiddleware: new writer level err: ", err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		defer func() {
			_ = gz.Close()
		}()

		w.Header().Set("Content-Encoding", "gzip")
		next.ServeHTTP(gzipWriter{ResponseWriter: w, Writer: gz}, r)
	})
}

func generateRandom(size int) ([]byte, error) {
	b := make([]byte, size)
	_, err := rand.Read(b)
	if err != nil {
		return nil, err
	}

	return b, nil
}

func makeUserIdentification() (string, error) {
	str := time.Now().Format("02012006150405")

	key, err := generateRandom(aes.BlockSize)
	if err != nil {
		return "", err
	}

	aesblock, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	aesgcm, err := cipher.NewGCM(aesblock)
	if err != nil {
		return "", err
	}

	nonce, err := generateRandom(aesgcm.NonceSize())
	if err != nil {
		return "", err
	}

	id := fmt.Sprintf("%x", aesgcm.Seal(nil, nonce, []byte(str), nil))

	return id, nil
}

var userIdentification = "user_identification"

var userLogin = "user_login"

var identification struct {
	cookie string
}

type cookieStruct struct {
	ID    string `json:"id"`
	Login string `json:"login"`
}

func (c *Controller) cookieMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var uid string

		cookie, err := r.Cookie(userIdentification)
		if err != nil {
			if !errors.Is(err, http.ErrNoCookie) {
				log.Print("cookieMiddleware: r.Cookie err: ", err.Error())
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			uid, err = makeUserIdentification()
			if err != nil {
				log.Print("cookieMiddleware: set user identification err: ", err.Error())
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			http.SetCookie(w, &http.Cookie{
				Name:     userIdentification,
				Value:    uid,
				Path:     "/",
				MaxAge:   3600,
				HttpOnly: false,
				Secure:   false,
				SameSite: http.SameSiteLaxMode,
			})
		} else {
			uid = cookie.Value
		}

		login, err := c.db.Authentication(uid)
		if err != nil {
			log.Print("cookieMiddleware: set user authentication err: ", err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:     userLogin,
			Value:    login,
			Path:     "/",
			MaxAge:   3600,
			HttpOnly: false,
			Secure:   false,
			SameSite: http.SameSiteLaxMode,
		})

		marshal, err := json.Marshal(cookieStruct{ID: uid, Login: login})
		if err != nil {
			log.Print("cookieMiddleware: marshal err: ", err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		ctx := context.WithValue(r.Context(), identification, marshal)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
