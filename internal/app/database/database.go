package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/chazari-x/yandex-pr-diplom/internal/app/config"
	_ "github.com/lib/pq"
)

type DataBase struct {
	asa     string
	DB      *sql.DB
	inputCh chan orderStr
}

var (
	Used             = errors.New("used")
	Empty            = errors.New("empty")
	NoMoney          = errors.New("no money")
	Duplicate        = errors.New("duplicate")
	WrongData        = errors.New("wrong data")
	BadOrderNumber   = errors.New("bad order number")
	RegisterConflict = errors.New("register conflict")
)

var dbCreateTables = `CREATE TABLE IF NOT EXISTS users (
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

var dbGetNotCheckedOrders = `SELECT number, status FROM orders WHERE status = 'NEW' OR status = 'PROCESSING'`

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

	log.Print("DB open")

	if _, err = db.Exec(dbCreateTables); err != nil {
		return nil, err
	}

	dBase := &DataBase{asa: c.AccrualSystemAddress, DB: db, inputCh: make(chan orderStr)}

	ctx, cancel = context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	rows, err := db.QueryContext(ctx, dbGetNotCheckedOrders)
	if err != nil {
		return nil, err
	}

	go func(rows *sql.Rows) {
		var orders []orderStr
		for rows.Next() {
			var order orderStr
			err := rows.Scan(&order.number, &order.status)
			if err != nil {
				log.Print(err)
				continue
			}

			orders = append(orders, order)
		}

		go func(orders []orderStr) {
			for _, order := range orders {
				dBase.inputCh <- order
			}
		}(orders)
	}(rows)

	dBase.newWorker(dBase.inputCh)

	return dBase, nil
}
