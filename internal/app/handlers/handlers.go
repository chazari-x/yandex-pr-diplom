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

var identification struct {
	ID string
}

func cookieMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var uid string

		cookie, err := r.Cookie(userIdentification)
		if err != nil {
			if !errors.Is(err, http.ErrNoCookie) {
				log.Print("cookieMiddleware: r.Cookie err: ", err.Error())
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			uid, err = setCookie(w)
			if err != nil {
				log.Print("cookieMiddleware: set user identification err: ", err.Error())
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

func (c *Controller) PostRegister(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	cookie := fmt.Sprintf("%v", r.Context().Value(identification))

	b, err := io.ReadAll(r.Body)
	if err != nil {
		log.Print("PostRegister: read all err: ", err.Error())
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
		log.Print("PostRegister: json unmarshal err: ", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = c.db.Register(user.Login, user.Password, cookie)
	if err != nil {
		if errors.Is(err, c.db.Err.RegisterConflict) {
			log.Printf("PostRegister: %d, cookie: %s, login: %s, password: %s",
				http.StatusConflict, cookie, user.Login, user.Password)
			w.WriteHeader(http.StatusConflict)
			return
		}

		log.Printf("PostRegister: %s, cookie: %s, login: %s, password: %s",
			err.Error(), cookie, user.Login, user.Password)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Authorization", user.Login)
	log.Printf("PostRegister: %d, cookie: %s, login: %s, password: %s",
		http.StatusOK, cookie, user.Login, user.Password)
	w.WriteHeader(http.StatusOK)
}

func (c *Controller) PostLogin(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	cookie := fmt.Sprintf("%v", r.Context().Value(identification))

	b, err := io.ReadAll(r.Body)
	if err != nil {
		log.Print("PostLogin: read all err: ", err.Error())
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
		log.Print("PostLogin: json unmarshal err: ", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	var status = http.StatusOK
	err = c.db.Login(user.Login, user.Password, cookie)
	if err != nil {
		if !errors.Is(err, c.db.Err.WrongData) {
			log.Printf("PostLogin: %s, login: %s, password: %s", err.Error(), user.Login, user.Password)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		status = http.StatusUnauthorized
	}

	w.Header().Set("Authorization", user.Login)
	log.Printf("PostLogin: %d, cookie: %s, login: %s, password: %s", status, cookie, user.Login, user.Password)
	w.WriteHeader(status)
}

func (c *Controller) PostOrders(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	cookie := fmt.Sprintf("%v", r.Context().Value(identification))

	b, err := io.ReadAll(r.Body)
	if err != nil {
		log.Print("PostOrders: read all err: ", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if string(b) == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	var order int
	err = json.Unmarshal(b, &order)
	if err != nil {
		log.Print("PostOrders: json unmarshal err: ", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = c.db.AddOrder(cookie, order)
	if err != nil {
		if errors.Is(err, c.db.Err.NoAuthorization) {
			log.Printf("PostOrders: %d, cookie: %s, order: %d", http.StatusUnauthorized, cookie, order)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		if errors.Is(err, c.db.Err.BadOrderNumber) {
			log.Printf("PostOrders: %d, cookie: %s, order: %d", http.StatusUnprocessableEntity, cookie, order)
			w.WriteHeader(http.StatusUnprocessableEntity)
			return
		}

		if errors.Is(err, c.db.Err.Duplicate) {
			log.Printf("PostOrders: %d, cookie: %s, order: %d", http.StatusOK, cookie, order)
			w.WriteHeader(http.StatusOK)
			return
		}

		if errors.Is(err, c.db.Err.Used) {
			log.Printf("PostOrders: %d, cookie: %s, order: %d", http.StatusConflict, cookie, order)
			w.WriteHeader(http.StatusConflict)
			return
		}

		log.Print("PostOrders: add order err: ", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	log.Printf("PostOrders: %d, cookie: %s, order: %d", http.StatusAccepted, cookie, order)
	w.WriteHeader(http.StatusAccepted)
}

func (c *Controller) GetOrders(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	cookie := fmt.Sprintf("%v", r.Context().Value(identification))

	orders, err := c.db.GetOrders(cookie)
	if err != nil {
		if errors.Is(err, c.db.Err.NoAuthorization) {
			log.Printf("GetOrders: %d, cookie: %s", http.StatusUnauthorized, cookie)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		if errors.Is(err, c.db.Err.Empty) {
			log.Printf("GetOrders: %d, cookie: %s", http.StatusNoContent, cookie)
			w.WriteHeader(http.StatusNoContent)
			return
		}

		log.Printf("GetOrders: %s, cookie: %s", err.Error(), cookie)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	marshal, err := json.Marshal(orders)
	if err != nil {
		log.Print("GetOrders: json marshal err: ", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	wr, err := w.Write(marshal)
	if err != nil {
		log.Print("GetOrders: w write err: ", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if wr == 0 {
		w.WriteHeader(http.StatusNoContent)
	}

	log.Printf("GetOrders: %d, cookie: %s", http.StatusOK, cookie)
}

func (c *Controller) GetBalance(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	cookie := fmt.Sprintf("%v", r.Context().Value(identification))

	balance, err := c.db.GetBalance(cookie)
	if err != nil {
		if errors.Is(err, c.db.Err.NoAuthorization) {
			log.Printf("GetBalance: %d, cookie: %s, current: %g, withdrawn: %g",
				http.StatusUnauthorized, cookie, balance.Current, balance.WithDraw)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		log.Printf("GetBalance: %s, cookie: %s, current: %g, withdrawn: %g",
			err.Error(), cookie, balance.Current, balance.WithDraw)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	marshal, err := json.Marshal(balance)
	if err != nil {
		log.Print("GetBalance: json marshal err: ", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	_, err = w.Write(marshal)
	if err != nil {
		log.Print("GetBalance: w write err: ", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	log.Printf("GetBalance: %d, cookie: %s, current: %g, withdrawn: %g",
		http.StatusOK, cookie, balance.Current, balance.WithDraw)
}

type withdraw struct {
	Order string  `json:"order"`
	Sum   float64 `json:"sum"`
}

func (c *Controller) PostWithDraw(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	cookie := fmt.Sprintf("%v", r.Context().Value(identification))

	b, err := io.ReadAll(r.Body)
	if err != nil {
		log.Print("PostWithDraw: read all err: ", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if string(b) == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	withdraw := withdraw{}
	err = json.Unmarshal(b, &withdraw)
	if err != nil {
		log.Print("PostWithDraw: json unmarshal err: ", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = c.db.AddWithDraw(cookie, withdraw.Order, withdraw.Sum)
	if err != nil {
		if errors.Is(err, c.db.Err.NoAuthorization) {
			log.Printf("PostWithDraw: %d, cookie: %s, order: %s, sum: %g",
				http.StatusUnauthorized, cookie, withdraw.Order, withdraw.Sum)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		if errors.Is(err, c.db.Err.NoMoney) {
			log.Printf("PostWithDraw: %d, cookie: %s, order: %s, sum: %g",
				http.StatusPaymentRequired, cookie, withdraw.Order, withdraw.Sum)
			w.WriteHeader(http.StatusPaymentRequired)
			return
		}

		if errors.Is(err, c.db.Err.BadOrderNumber) {
			log.Printf("PostWithDraw: %d, cookie: %s, order: %s, sum: %g",
				http.StatusUnprocessableEntity, cookie, withdraw.Order, withdraw.Sum)
			w.WriteHeader(http.StatusUnprocessableEntity)
			return
		}

		log.Printf("PostWithDraw: %s, cookie: %s, order: %s, sum: %g",
			err.Error(), cookie, withdraw.Order, withdraw.Sum)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	log.Printf("PostWithDraw: %d, cookie: %s, order: %s, sum: %g",
		http.StatusOK, cookie, withdraw.Order, withdraw.Sum)
	w.WriteHeader(http.StatusOK)
}

func (c *Controller) GetWithDrawAls(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	cookie := fmt.Sprintf("%v", r.Context().Value(identification))

	withdraw, err := c.db.GetWithDraw(cookie)
	if err != nil {
		if errors.Is(err, c.db.Err.NoAuthorization) {
			log.Printf("GetWithDraw: %d, cookie: %s", http.StatusUnauthorized, cookie)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		if errors.Is(err, c.db.Err.Empty) {
			log.Printf("GetWithDraw: %d, cookie: %s", http.StatusNoContent, cookie)
			w.WriteHeader(http.StatusNoContent)
			return
		}

		log.Print("GetWithDraw: add order err: ", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	marshal, err := json.Marshal(withdraw)
	if err != nil {
		log.Print("GetWithDraw: json marshal err: ", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	_, err = w.Write(marshal)
	if err != nil {
		log.Print("GetWithDraw: w write err: ", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	log.Printf("GetWithDraw: %d, cookie: %s", http.StatusOK, cookie)
}
