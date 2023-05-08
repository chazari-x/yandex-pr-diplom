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

	"github.com/chazari-x/yandex-pr-diplom/internal/app/config"
	"github.com/chazari-x/yandex-pr-diplom/internal/app/database"
)

type Controller struct {
	c  config.Config
	db *database.DataBase
}

func NewController(c config.Config, db *database.DataBase) *Controller {
	return &Controller{c: c, db: db}
}

type Middleware func(http.Handler) http.Handler

func MiddlewaresConveyor(h http.Handler) http.Handler {
	middlewares := []Middleware{gzipMiddleware, cookieMiddleware}
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
				log.Print("GZIP: new reader err: ", err)
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
			log.Print("GZIP: new writer level err: ", err)
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

var identification struct {
	ID string
}

func cookieMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var uid string

		cookie, err := r.Cookie(userIdentification)
		if err != nil {
			if !errors.Is(err, http.ErrNoCookie) {
				log.Print("COOKIE: r.Cookie err: ", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			uid, err = setCookie(w)
			if err != nil {
				log.Print("COOKIE: set user identification err: ", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		} else {
			uid = cookie.Value
		}

		ctx := context.WithValue(r.Context(), identification, uid)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func setCookie(w http.ResponseWriter) (string, error) {
	uid, err := makeUserIdentification()
	if err != nil {
		return "", err
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

	return uid, nil
}

type userStruct struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

func (c *Controller) Register(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	uid := fmt.Sprintf("%v", r.Context().Value(identification))

	b, err := io.ReadAll(r.Body)
	if err != nil {
		log.Print("REGISTER: read all err: ", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if string(b) == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	user := userStruct{}

	err = json.Unmarshal(b, &user)
	if err != nil {
		log.Print("REGISTER: json unmarshal err: ", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	var status = http.StatusOK

	for i := 0; i < 2; i++ {
		err = c.db.Register(user.Login, user.Password, uid)
		if err == nil {
			break
		}

		if strings.Contains(err.Error(), "register conflict") {
			status = http.StatusConflict
			break
		}

		if !strings.Contains(err.Error(), "duplicate identification") {
			log.Printf("register: %s, login: %s, password: %s", err, user.Login, user.Password)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		uid, err = setCookie(w)
		if err != nil {
			log.Print("REGISTER: set cookie err: ", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}

	log.Printf("register: %d, id: %s, login: %s, password: %s", status, uid, user.Login, user.Password)
	w.WriteHeader(status)
}

func (c *Controller) Login(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	uid := fmt.Sprintf("%v", r.Context().Value(identification))

	b, err := io.ReadAll(r.Body)
	if err != nil {
		log.Print("LOGIN: read all err: ", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if string(b) == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	user := userStruct{}

	err = json.Unmarshal(b, &user)
	if err != nil {
		log.Print("LOGIN: json unmarshal err: ", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	var status = http.StatusOK

	dbUID, err := c.db.Login(user.Login, user.Password)
	if err != nil {
		if !strings.Contains(err.Error(), "empty") {
			log.Printf("login: %s, login: %s, password: %s", err, user.Login, user.Password)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		status = http.StatusUnauthorized
	}

	if dbUID != "" && dbUID != uid {
		http.SetCookie(w, &http.Cookie{
			Name:     userIdentification,
			Value:    dbUID,
			Path:     "/",
			MaxAge:   3600,
			HttpOnly: false,
			Secure:   false,
			SameSite: http.SameSiteLaxMode,
		})

		uid = dbUID
	}

	log.Printf("login: %d, id: %s, login: %s, password: %s", status, uid, user.Login, user.Password)
	w.WriteHeader(status)
}
