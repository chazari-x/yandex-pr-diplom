package database

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"
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
	dbGetOrders     = `SELECT number, status, COALESCE(accrual, 0), uploaded_at FROM orders WHERE login = $1`
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

func (db *DataBase) AddOrder(login string, order int) error {
	if !checkOrderNumber(order) {
		return BadOrderNumber
	}

	exec, err := db.DB.Exec(dbAddOrder, order, login, time.Now().Format(time.RFC3339))
	if err != nil {
		return err
	}

	affected, err := exec.RowsAffected()
	if err != nil {
		return err
	}

	if affected != 0 {
		db.getOrderInfo(strconv.Itoa(order))
		return nil
	}

	var orderLogin string
	if err = db.DB.QueryRow(dbGetOrderLogin, order).Scan(&orderLogin); err != nil {
		return err
	}

	if orderLogin != login {
		return Used
	}

	return Duplicate
}

type orderStr struct {
	number string
	status string
}

func (db *DataBase) getOrderInfo(number string) {
	go func(number string) {
		db.inputCh <- orderStr{number: number, status: "NEW"}
	}(number)
}

func (db *DataBase) newWorker(input chan orderStr) {
	go func() {
		log.Print("starting goroutine")

		defer func() {
			db.newWorker(input)
			if x := recover(); x != nil {
				log.Print("run time panic: ", x)
			}
		}()

		for {
			for o := range input {
				resp, err := http.Get(db.asa + "/api/orders/" + o.number)
				if err != nil {
					go func(o orderStr) {
						db.inputCh <- o
					}(o)
					log.Printf("go number: %s, err: %s", o.number, err.Error())
					resp.Body.Close()
					continue
				}

				b, err := io.ReadAll(resp.Body)
				if err != nil {
					go func(o orderStr) {
						db.inputCh <- o
					}(o)
					log.Printf("go number: %s, err: %s", o.number, err.Error())
					resp.Body.Close()
					continue
				}

				resp.Body.Close()

				switch resp.StatusCode {
				case http.StatusOK:
					var order Order
					err = json.Unmarshal(b, &order)
					if err != nil {
						go func(o orderStr) {
							db.inputCh <- o
						}(o)
						log.Printf("go number: %s, err: %s", o.number, err.Error())
						continue
					}

					order.Number = o.number

					switch order.Status {
					case "PROCESSING":
						log.Printf("go number: %s, status: %s", o.number, order.Status)
						go func(o orderStr, order Order) {
							if o.status != order.Status {
								err := db.updateOrder(order)
								if err != nil {
									log.Printf("go number: %s, err: %s", o.number, err.Error())
									return
								}
								o.status = "PROCESSING"
							}
							db.inputCh <- o
						}(o, order)
					case "INVALID", "PROCESSED":
						log.Printf("go number: %s, status: %s, accrual: %g", o.number, order.Status, order.Accrual)
						go func(o orderStr, order Order) {
							if o.status != order.Status {
								err := db.updateOrder(order)
								if err != nil {
									o.status = order.Status
									db.inputCh <- o
									log.Printf("go number: %s, err: %s", o.number, err.Error())
									return
								}
							}
						}(o, order)
					default:
						log.Printf("go number: %s, status: %s", o.number, order.Status)
						go func(o orderStr) {
							db.inputCh <- o
						}(o)
					}
				case http.StatusTooManyRequests:
					log.Printf("go number: %s, status: %s", o.number, resp.Status)
					go func(o orderStr) {
						db.inputCh <- o
					}(o)
					atoi, err := strconv.Atoi(resp.Header.Get("Retry-After"))
					if err != nil {
						log.Printf("go number: %s, err: %s", o.number, err.Error())
						time.Sleep(time.Second * 15)
					} else {
						time.Sleep(time.Second * time.Duration(atoi))
					}
				case http.StatusInternalServerError:
					log.Printf("go number: %s, status: %s", o.number, resp.Status)
					go func(o orderStr) {
						db.inputCh <- o
					}(o)
				case http.StatusNoContent:
					log.Printf("go number: %s, status: %s", o.number, resp.Status)
					go func(o orderStr) {
						if o.status != "PROCESSING" {
							err := db.updateOrder(Order{Status: "PROCESSING", Number: o.number})
							if err != nil {
								log.Printf("go number: %s, err: %s", o.number, err.Error())
								return
							}
							o.status = "PROCESSING"
						}
						db.inputCh <- o
					}(o)
				default:
					log.Printf("go number: %s, status: %s", o.number, resp.Status)
					go func(o orderStr) {
						db.inputCh <- o
					}(o)
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

func (db *DataBase) GetOrders(login string) ([]Order, error) {
	rows, err := db.DB.Query(dbGetOrders, login)
	if err != nil {
		return nil, err
	}

	var orders []Order
	for rows.Next() {
		var order Order
		if err = rows.Scan(&order.Number, &order.Status, &order.Accrual, &order.UploadedAt); err != nil {
			return nil, err
		}

		orders = append(orders, order)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	if orders == nil {
		return nil, Empty
	}

	return orders, nil
}
