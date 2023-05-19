package database

import (
	"context"
	"errors"
	"log"
	"strconv"
	"time"

	"github.com/chazari-x/yandex-pr-diplom/internal/app/worker"
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
	dbAddOrder            = `INSERT INTO orders (number, login, uploaded_at) VALUES ($1, $2, $3) ON CONFLICT(number) DO NOTHING`
	dbGetOrders           = `SELECT number, status, COALESCE(accrual, 0), uploaded_at FROM orders WHERE login = $1`
	dbGetNotCheckedOrders = `SELECT number FROM orders WHERE status = 'NEW' OR status = 'PROCESSING'`
	dbUpdateOrder         = `UPDATE orders SET status = $1, accrual = $2 WHERE number = $3`
	dbGetOrderLogin       = `SELECT login FROM orders WHERE number = $1`
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
		return ErrBadOrderNumber
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
		worker.AddOrder(strconv.Itoa(order))
		return nil
	}

	var orderLogin string
	if err = db.DB.QueryRow(dbGetOrderLogin, order).Scan(&orderLogin); err != nil {
		return err
	}

	if orderLogin != login {
		return ErrUsed
	}

	return ErrDuplicate
}

func (db *DataBase) GetNotCheckedOrders() ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	rows, err := db.DB.QueryContext(ctx, dbGetNotCheckedOrders)
	if err != nil {
		return nil, err
	}

	var orders []string
	for rows.Next() {
		var order string
		err := rows.Scan(&order)
		if err != nil {
			log.Print(err)
			continue
		}

		orders = append(orders, order)
	}

	if err := rows.Err(); err != nil {
		log.Print(err)
		return nil, err
	}

	return orders, nil
}

func (db *DataBase) UpdateOrder(number, status string, accrual float64) error {
	exec, err := db.DB.Exec(dbUpdateOrder, status, accrual, number)
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

	log.Printf("update order: number: %s, status: %s, accrual: %g", number, status, accrual)

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
		return nil, ErrEmpty
	}

	return orders, nil
}
