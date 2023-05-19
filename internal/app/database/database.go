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
	DB *sql.DB
}

var (
	ErrUsed             = errors.New("used")
	ErrEmpty            = errors.New("empty")
	ErrNoMoney          = errors.New("no money")
	ErrDuplicate        = errors.New("duplicate")
	ErrWrongData        = errors.New("wrong data")
	ErrBadOrderNumber   = errors.New("bad order number")
	ErrRegisterConflict = errors.New("register conflict")
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

	ctx, cancel = context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if _, err = db.ExecContext(ctx, dbCreateTables); err != nil {
		return nil, err
	}

	return &DataBase{DB: db}, nil
}
