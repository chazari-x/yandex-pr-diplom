package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/chazari-x/yandex-pr-diplom/internal/app/database"
)

func (c *Controller) GetOrders(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var cookie cookieStruct
	err := json.Unmarshal([]byte(fmt.Sprintf("%s", r.Context().Value(identification))), &cookie)
	if err != nil {
		log.Print("PostRegister: unmarshal cookie err: ", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if cookie.Login == "" {
		log.Printf("GetOrders: %d, cookie: %s", http.StatusUnauthorized, cookie)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	orders, err := c.db.GetOrders(cookie.Login)
	if err != nil {
		if errors.Is(err, database.Empty) {
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

	var cookie cookieStruct
	err := json.Unmarshal([]byte(fmt.Sprintf("%s", r.Context().Value(identification))), &cookie)
	if err != nil {
		log.Print("PostRegister: unmarshal cookie err: ", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if cookie.Login == "" {
		log.Printf("GetBalance: %d, cookie: %s",
			http.StatusUnauthorized, cookie)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	balance, err := c.db.GetBalance(cookie.Login)
	if err != nil {
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

func (c *Controller) GetWithDrawAls(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var cookie cookieStruct
	err := json.Unmarshal([]byte(fmt.Sprintf("%s", r.Context().Value(identification))), &cookie)
	if err != nil {
		log.Print("PostRegister: unmarshal cookie err: ", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if cookie.Login == "" {
		log.Printf("GetWithDraw: %d, cookie: %s", http.StatusUnauthorized, cookie)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	withdraw, err := c.db.GetWithDraw(cookie.Login)
	if err != nil {
		if errors.Is(err, database.Empty) {
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
