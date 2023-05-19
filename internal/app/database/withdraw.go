package database

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"strings"
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
	dbGetWithDraw = `SELECT orderID, sum, processed_at FROM withdraw WHERE login = $1`
	dbAddWithDraw = `INSERT INTO withdraw SELECT $1, $2, $3, $4
						WHERE NOT COALESCE((SELECT SUM(accrual) FROM orders WHERE login = $5 GROUP BY login), 0) -
						COALESCE((SELECT SUM(sum) FROM withdraw WHERE login = $5 GROUP BY login), 0) - $3 < 0`
)

func (db *DataBase) AddWithDraw(login, order string, sum float64) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	log.Print(login)

	exec, err := db.DB.ExecContext(ctx, dbAddWithDraw, order, login, sum, time.Now().Format(time.RFC3339), login)
	if err != nil {
		if !strings.Contains(err.Error(), "duplicate key value violates unique constraint \"withdraw_pkey\"") {
			return err
		}

		return ErrBadOrderNumber
	}

	affected, err := exec.RowsAffected()
	if err != nil {
		return err
	}

	if affected == 0 {
		return ErrNoMoney
	}

	return nil
}

func (db *DataBase) GetWithDraw(login string) ([]WithDraw, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	rows, err := db.DB.QueryContext(ctx, dbGetWithDraw, login)
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
		return nil, ErrEmpty
	}

	return withdraw, nil
}
