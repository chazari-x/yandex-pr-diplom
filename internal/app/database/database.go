package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
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
	Used             error
	Empty            error
	NoMoney          error
	Duplicate        error
	WrongData        error
	BadOrderNumber   error
	NoAuthorization  error
	RegisterConflict error
}

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
