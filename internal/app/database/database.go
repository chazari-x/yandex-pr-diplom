package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/chazari-x/yandex-pr-diplom/internal/app/config"
	_ "github.com/lib/pq"
)

type DataBase struct {
	ASA string
	DB  *sql.DB
	Err errs
}

type errs struct {
	RegisterConflict error
	Empty            error
	Duplicate        error
	NoAuthorization  error
	Used             error
	NoMoney          error
	WrongData        error
	BadOrderNumber   error
}

type User struct {
	UserID   string  `json:"user_id,omitempty"`
	Login    string  `json:"login,omitempty"`
	Password string  `json:"password,omitempty"`
	Cookie   string  `json:"cookie,omitempty"`
	Current  float64 `json:"current"`
	WithDraw float64 `json:"withdrawn"`
}

type Order struct {
	Number     string  `json:"number"`
	Login      string  `json:"login,omitempty"`
	Status     string  `json:"status"`
	Accrual    float64 `json:"accrual,omitempty"`
	UploadedAt string  `json:"uploaded_at,omitempty"`
}

type WithDraw struct {
	OrderID     string  `json:"order"`
	Login       string  `json:"login,omitempty"`
	Sum         float64 `json:"sum"`
	ProcessedAt string  `json:"processed_at"`
}

var (
	dbCreateTables = `CREATE TABLE IF NOT EXISTS users (
							userid			SERIAL  PRIMARY KEY NOT NULL,
							login			VARCHAR UNIQUE		NOT NULL,
							password		VARCHAR 			NOT NULL,
							cookie			VARCHAR UNIQUE		NULL);
	
					CREATE TABLE IF NOT EXISTS orders (
							number 			VARCHAR PRIMARY KEY NOT NULL,
							login 			VARCHAR 			NOT NULL,
							status 			VARCHAR 			NOT NULL	DEFAULT 'NEW',
							accrual 		NUMERIC 			NULL,
							uploaded_at 	VARCHAR				NOT NULL);
	
					CREATE TABLE IF NOT EXISTS withdraw (
							orderID 		VARCHAR PRIMARY KEY NOT NULL,
							login 			VARCHAR 			NOT NULL,
							sum 			NUMERIC 			NOT NULL,
							processed_at	VARCHAR 			NOT NULL);`

	// Таблица пользователей users:
	dbRegistration  = `INSERT INTO users (login, password, cookie) VALUES ($1, $2, $3) ON CONFLICT(login) DO NOTHING`
	dbAuthorization = `SELECT cookie FROM users WHERE login = $1 AND password = $2`
	dbGetLogin      = `SELECT login FROM users WHERE cookie = $1`
	dbGetBalance    = `SELECT login, 
						(SELECT SUM(accrual) FROM orders WHERE login = $1 GROUP BY login) -
						GREATEST(0, (SELECT SUM(sum) FROM withdraw WHERE login = $1 GROUP BY login)),
						(SELECT SUM(sum) FROM withdraw WHERE login = $1 GROUP BY login) 
						FROM users WHERE login = $1`
	dbDellCookie = `UPDATE users SET cookie = NULL WHERE cookie = $1`
	dbSetCookie  = `UPDATE users SET cookie = $1 WHERE login = $2 AND password = $3`

	// Таблица заказов orders:
	dbAddOrder      = `INSERT INTO orders (number, login, uploaded_at) VALUES ($1, $2, $3) ON CONFLICT(number) DO NOTHING`
	dbGetOrders     = `SELECT number, status, accrual, uploaded_at FROM orders WHERE login = $1`
	dbGetOrderLogin = `SELECT login FROM orders WHERE number = $1`
	dbUpdateOrder   = `UPDATE orders SET status = $1, accrual = $2 WHERE number = $3`

	// Таблица операций withdraw:
	dbAddWithDraw = `INSERT INTO withdraw VALUES ($1, $2, $3, $4) ON CONFLICT(orderID) DO NOTHING`
	dbGetWithDraw = `SELECT orderID, sum, processed_at FROM withdraw WHERE login = $1`
)

func StartDB(c config.Config) (*DataBase, error) {
	db, err := sql.Open("postgres", c.DataBaseURI)
	if err != nil {
		return nil, fmt.Errorf("sql open err: %s", err.Error())
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err = db.PingContext(ctx); err != nil {
		return nil, err
	}

	if _, err = db.Exec(dbCreateTables); err != nil {
		return nil, err
	}

	var errs errs
	errs.Used = errors.New("used")
	errs.Empty = errors.New("empty")
	errs.NoMoney = errors.New("no money")
	errs.Duplicate = errors.New("duplicate")
	errs.WrongData = errors.New("wrong data")
	errs.BadOrderNumber = errors.New("bad order number")
	errs.NoAuthorization = errors.New("no authorization")
	errs.RegisterConflict = errors.New("register conflict")

	return &DataBase{ASA: c.AccrualSystemAddress, DB: db, Err: errs}, nil
}

func (db *DataBase) Register(login, pass, cookie string) error {
	exec, err := db.DB.Exec(dbRegistration, login, pass, cookie)
	if err != nil {
		if !strings.Contains(err.Error(), "duplicate key value violates unique constraint \"users_cookie_key\"") {
			return err
		}

		if _, err = db.DB.Exec(dbDellCookie, cookie); err != nil {
			return err
		}

		if _, err = db.DB.Exec(dbRegistration, login, pass, cookie); err != nil {
			return err
		}

		return nil
	}

	affected, err := exec.RowsAffected()
	if err != nil {
		return err
	}

	if affected == 0 {
		return db.Err.RegisterConflict
	}

	return nil
}

func (db *DataBase) Login(login, pass, cookie string) error {
	var cookieDB string
	if err := db.DB.QueryRow(dbAuthorization, login, pass).Scan(&cookieDB); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return db.Err.WrongData
		}

		if !strings.Contains(err.Error(), "name \"cookie\": converting NULL to string is unsupported") {
			return err
		}
	}

	if cookieDB != cookie {
		if _, err := db.DB.Exec(dbDellCookie, cookie); err != nil {
			return err
		}

		if _, err := db.DB.Exec(dbSetCookie, cookie, login, pass); err != nil {
			return err
		}
	}

	return nil
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

func (db *DataBase) GetBalance(cookie string) (User, error) {
	var login string
	if err := db.DB.QueryRow(dbGetLogin, cookie).Scan(&login); err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return User{}, err
		}

		return User{}, db.Err.NoAuthorization
	}

	var balance User
	var current sql.NullFloat64
	var withdraw sql.NullFloat64
	if err := db.DB.QueryRow(dbGetBalance, login).Scan(&balance.Login, &current, &withdraw); err != nil {
		return User{}, err
	}

	balance.Current = current.Float64
	balance.WithDraw = withdraw.Float64

	return balance, nil
}

func (db *DataBase) AddWithDraw(cookie, order string, sum float64) error {
	var login string
	if err := db.DB.QueryRow(dbGetLogin, cookie).Scan(&login); err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}

		return db.Err.NoAuthorization
	}

	var balance User
	var current sql.NullFloat64
	var withdraw sql.NullFloat64
	if err := db.DB.QueryRow(dbGetBalance, login).Scan(&balance.Login, &current, &withdraw); err != nil {
		return err
	}

	balance.Current = current.Float64
	if balance.Current < sum {
		return db.Err.NoMoney
	}

	exec, err := db.DB.Exec(dbAddWithDraw, order, balance.Login, sum, time.Now().Format(time.RFC3339))
	if err != nil {
		return err
	}

	affected, err := exec.RowsAffected()
	if err != nil {
		return err
	}

	if affected == 0 {
		return db.Err.BadOrderNumber
	}

	return nil
}

func (db *DataBase) GetWithDraw(cookie string) ([]WithDraw, error) {
	var login string
	if err := db.DB.QueryRow(dbGetLogin, cookie).Scan(&login); err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}

		return nil, db.Err.NoAuthorization
	}

	rows, err := db.DB.Query(dbGetWithDraw, login)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
	}

	var withdraw []WithDraw
	for rows.Next() {
		var order WithDraw
		if err = rows.Scan(&order.OrderID, &order.Sum, &order.ProcessedAt); err != nil {
			if !errors.Is(err, sql.ErrNoRows) {
				return nil, err
			}
		}

		withdraw = append(withdraw, order)
	}

	if rows.Err() != nil {
		return nil, err
	}

	if withdraw == nil {
		return nil, db.Err.Empty
	}

	return withdraw, nil
}
