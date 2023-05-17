package database

import (
	"database/sql"
	"errors"
	"time"
)

type WithDraw struct {
	OrderID     string  `json:"order"`
	Login       string  `json:"login,omitempty"`
	Sum         float64 `json:"sum"`
	ProcessedAt string  `json:"processed_at"`
}

var (
	// Таблица операций withdraw:
	dbAddWithDraw = `INSERT INTO withdraw VALUES ($1, $2, $3, $4) ON CONFLICT(orderID) DO NOTHING`
	dbGetWithDraw = `SELECT orderID, sum, processed_at FROM withdraw WHERE login = $1`
)

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

	if err = rows.Err(); err != nil {
		return nil, err
	}

	if withdraw == nil {
		return nil, db.Err.Empty
	}

	return withdraw, nil
}
