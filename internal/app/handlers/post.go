package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/chazari-x/yandex-pr-diplom/internal/app/database"
)

type userStruct struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

func (c *Controller) PostRegister(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var cookie cookieStruct
	err := json.Unmarshal([]byte(fmt.Sprintf("%s", r.Context().Value(identification))), &cookie)
	if err != nil {
		log.Print("PostRegister: unmarshal cookie err: ", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

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

	err = c.db.Register(user.Login, user.Password, cookie.ID)
	if err != nil {
		if errors.Is(err, database.ErrRegisterConflict) {
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

	var cookie cookieStruct
	err := json.Unmarshal([]byte(fmt.Sprintf("%s", r.Context().Value(identification))), &cookie)
	if err != nil {
		log.Print("PostLogin: unmarshal cookie err: ", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

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
	err = c.db.Login(user.Login, user.Password, cookie.ID)
	if err != nil {
		if !errors.Is(err, database.ErrWrongData) {
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

	var cookie cookieStruct
	err := json.Unmarshal([]byte(fmt.Sprintf("%s", r.Context().Value(identification))), &cookie)
	if err != nil {
		log.Print("PostRegister: unmarshal cookie err: ", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if cookie.Login == "" {
		log.Printf("PostOrders: %d, cookie: %s", http.StatusUnauthorized, cookie)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

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

	err = c.db.AddOrder(cookie.Login, order)
	if err != nil {
		if errors.Is(err, database.ErrBadOrderNumber) {
			log.Printf("PostOrders: %d, cookie: %s, order: %d", http.StatusUnprocessableEntity, cookie, order)
			w.WriteHeader(http.StatusUnprocessableEntity)
			return
		}

		if errors.Is(err, database.ErrDuplicate) {
			log.Printf("PostOrders: %d, cookie: %s, order: %d", http.StatusOK, cookie, order)
			w.WriteHeader(http.StatusOK)
			return
		}

		if errors.Is(err, database.ErrUsed) {
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

type withdraw struct {
	Order string  `json:"order"`
	Sum   float64 `json:"sum"`
}

func (c *Controller) PostWithDraw(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var cookie cookieStruct
	err := json.Unmarshal([]byte(fmt.Sprintf("%s", r.Context().Value(identification))), &cookie)
	if err != nil {
		log.Print("PostRegister: unmarshal cookie err: ", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if cookie.Login == "" {
		log.Printf("PostWithDraw: %d, cookie: %s",
			http.StatusUnauthorized, cookie)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

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

	err = c.db.AddWithDraw(cookie.Login, withdraw.Order, withdraw.Sum)
	if err != nil {
		if errors.Is(err, database.ErrNoMoney) {
			log.Printf("PostWithDraw: %d, cookie: %s, order: %s, sum: %g",
				http.StatusPaymentRequired, cookie, withdraw.Order, withdraw.Sum)
			w.WriteHeader(http.StatusPaymentRequired)
			return
		}

		if errors.Is(err, database.ErrBadOrderNumber) {
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
