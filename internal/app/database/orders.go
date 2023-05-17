package database

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Order struct {
	Number     string  `json:"number"`
	Login      string  `json:"login,omitempty"`
	Status     string  `json:"status"`
	Accrual    float64 `json:"accrual,omitempty"`
	UploadedAt string  `json:"uploaded_at,omitempty"`
}

var (
	// Таблица заказов orders:
	dbAddOrder      = `INSERT INTO orders (number, login, uploaded_at) VALUES ($1, $2, $3) ON CONFLICT(number) DO NOTHING`
	dbGetOrders     = `SELECT number, status, accrual, uploaded_at FROM orders WHERE login = $1`
	dbUpdateOrder   = `UPDATE orders SET status = $1, accrual = $2 WHERE number = $3`
	dbGetOrderLogin = `SELECT login FROM orders WHERE number = $1`
)

var t = [...]int{0, 2, 4, 6, 8, 1, 3, 5, 7, 9}

func checkOrderNumber(number int) bool {
	s := strconv.Itoa(number)
	odd := len(s) & 1
	var sum int
	for i, c := range s {
		if c < '0' || c > '9' {
			return false
		}
		if i&1 == odd {
			sum += t[c-'0']
		} else {
			sum += int(c - '0')
		}
	}
	return sum%10 == 0
}

func (db *DataBase) AddOrder(cookie string, order int) error {
	var login string
	if err := db.DB.QueryRow(dbGetLogin, cookie).Scan(&login); err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}

		return db.Err.NoAuthorization
	}

	if !checkOrderNumber(order) {
		return db.Err.BadOrderNumber
	}

	exec, err := db.DB.Exec(dbAddOrder, order, login, time.Now().Format(time.RFC3339))
	if err != nil {
		return err
	}

	affected, err := exec.RowsAffected()
	if err != nil {
		return err
	}

	if affected == 0 {
		var orderLogin string
		if err = db.DB.QueryRow(dbGetOrderLogin, order).Scan(&orderLogin); err != nil {
			return err
		}

		if orderLogin != login {
			return db.Err.Used
		}

		return db.Err.Duplicate
	}

	db.getOrderInfo(strconv.Itoa(order))

	return nil
}

const workersCount = 1

var workers = 0

var inputCh = make(chan string)

func (db *DataBase) getOrderInfo(number string) {
	go func(number string) {
		inputCh <- number
	}(number)

	if workers < workersCount {
		for i := workers; i < workersCount; i++ {
			workers++
			db.newWorker(inputCh)
		}
	}
}

func (db *DataBase) newWorker(input chan string) {
	go func() {
		log.Print("starting goroutine")

		defer func() {
			db.newWorker(input)
			if x := recover(); x != nil {
				log.Print("run time panic: ", x)
			}
		}()

		for {
			for number := range input {
				resp, err := http.Get(db.ASA + "/api/orders/" + number)
				if err != nil {
					go func(number string) {
						inputCh <- number
					}(number)
					log.Printf("go number: %s, err: %s", number, err.Error())
					resp.Body.Close()
					return
				}

				b, err := io.ReadAll(resp.Body)
				if err != nil {
					go func(number string) {
						inputCh <- number
					}(number)
					log.Printf("go number: %s, err: %s", number, err.Error())
					resp.Body.Close()
					return
				}

				resp.Body.Close()

				var status = 0
				if strings.Contains(resp.Status, "200") {
					status = http.StatusOK
				} else if strings.Contains(resp.Status, "429") {
					status = http.StatusTooManyRequests
				} else if strings.Contains(resp.Status, "500") {
					status = http.StatusInternalServerError
				} else if strings.Contains(resp.Status, "204") {
					status = http.StatusNoContent
				}

				switch status {
				case http.StatusOK:
					var order Order
					err = json.Unmarshal(b, &order)
					if err != nil {
						go func(number string) {
							inputCh <- number
						}(number)
						log.Printf("go number: %s, err: %s", number, err.Error())
						return
					}

					order.Number = number

					switch order.Status {
					case "PROCESSING":
						log.Printf("go number: %s, status: %s", number, order.Status)
						go func(number string, order Order) {
							err := db.updateOrder(order)
							if err != nil {
								log.Printf("go number: %s, err: %s", number, err.Error())
								return
							}
							inputCh <- number
						}(number, order)
					case "INVALID", "PROCESSED":
						log.Printf("go number: %s, status: %s, accrual: %g", number, order.Status, order.Accrual)
						go func(number string, order Order) {
							err := db.updateOrder(order)
							if err != nil {
								inputCh <- number
								log.Printf("go number: %s, err: %s", number, err.Error())
								return
							}
						}(number, order)
					default:
						log.Printf("go number: %s, status: %s", number, order.Status)
						go func(number string) {
							inputCh <- number
						}(number)
					}
				case http.StatusTooManyRequests:
					log.Printf("go number: %s, status: %s", number, resp.Status)
					go func(number string) {
						inputCh <- number
					}(number)
					atoi, err := strconv.Atoi(resp.Header.Get("Retry-After"))
					if err != nil {
						log.Printf("go number: %s, err: %s", number, err.Error())
						time.Sleep(time.Second * 15)
					} else {
						time.Sleep(time.Second * time.Duration(atoi))
					}
				case http.StatusInternalServerError:
					log.Printf("go number: %s, status: %s", number, resp.Status)
					go func(number string) {
						inputCh <- number
					}(number)
				case http.StatusNoContent:
					log.Printf("go number: %s, status: %s", number, resp.Status)
					go func(number string) {
						err := db.updateOrder(Order{Status: "PROCESSING", Number: number})
						if err != nil {
							log.Printf("go number: %s, err: %s", number, err.Error())
							return
						}
						inputCh <- number
					}(number)
				default:
					log.Printf("go number: %s, status: %s", number, resp.Status)
					go func(number string) {
						inputCh <- number
					}(number)
				}
			}
		}
	}()
}

func (db *DataBase) updateOrder(order Order) error {
	exec, err := db.DB.Exec(dbUpdateOrder, order.Status, order.Accrual, order.Number)
	if err != nil {
		return err
	}

	affected, err := exec.RowsAffected()
	if err != nil {
		return err
	}

	if affected == 0 {
		return errors.New("failed update order")
	}

	log.Printf("update order: number: %s, status: %s, accrual: %g", order.Number, order.Status, order.Accrual)

	return nil
}

func (db *DataBase) GetOrders(cookie string) ([]Order, error) {
	var login string
	if err := db.DB.QueryRow(dbGetLogin, cookie).Scan(&login); err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}

		return nil, db.Err.NoAuthorization
	}

	rows, err := db.DB.Query(dbGetOrders, login)
	if err != nil {
		return nil, err
	}

	var orders []Order
	for rows.Next() {
		var order Order
		var accrual sql.NullFloat64
		if err = rows.Scan(&order.Number, &order.Status, &accrual, &order.UploadedAt); err != nil {
			return nil, err
		}

		order.Accrual = accrual.Float64

		orders = append(orders, order)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	if orders == nil {
		return nil, db.Err.Empty
	}

	return orders, nil
}
