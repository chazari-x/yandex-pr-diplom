package database

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/chazari-x/yandex-pr-diplom/internal/app/config"
)

type DBStorage interface {
	Register(login, pass string) error
}

type DataBase struct {
	DB  *sql.DB
	Err errs
}

type errs struct {
	RegisterConflict error
	Empty            error
	Duplicate        error
	NoAuthorization  error
	Used             error
}

type User struct {
	UserID   string `json:"user_id"`
	Login    string `json:"login"`
	Password string `json:"password"`
	Cookie   string `json:"cookie,omitempty"`
	Current  int    `json:"current"`
	WithDraw int    `json:"withdraw"`
}

type Order struct {
	Number     string `json:"number"`
	Login      string `json:"login"`
	Status     string `json:"status"`
	Accrual    int    `json:"accrual,omitempty"`
	UploadedAt string `json:"uploaded_at"`
}

type WithDraw struct {
	OrderID     string `json:"order_id"`
	Login       string `json:"login"`
	Sum         int    `json:"sum"`
	ProcessedAt string `json:"processed_at"`
}

var (
	dbCreateUsersTable = `CREATE TABLE IF NOT EXISTS users (
							userid			SERIAL  PRIMARY KEY NOT NULL,
							login			VARCHAR UNIQUE		NOT NULL, 
							password		VARCHAR 			NOT NULL, 
							cookie			VARCHAR UNIQUE		NULL,
							current			INTEGER 			NOT NULL	DEFAULT 0, 
							withdraw		INTEGER 			NOT NULL	DEFAULT 0)`

	dbCreateOrdersTable = `CREATE TABLE IF NOT EXISTS Orders (
							number 			VARCHAR PRIMARY KEY NOT NULL,
							login 			VARCHAR 			NOT NULL,
							status 			VARCHAR 			NOT NULL	DEFAULT 'NEW', 
							accrual 		INTEGER 			NULL,
							uploaded_at 	VARCHAR				NOT NULL)`

	dbCreateWithDrawTable = `CREATE TABLE IF NOT EXISTS withdraw (
							orderID 		VARCHAR PRIMARY KEY NOT NULL,
							login 			VARCHAR 			NOT NULL,
							sum 			INTEGER 			NOT NULL,
							processed_at	VARCHAR 			NOT NULL)`

	// Таблица пользователей users:
	dbRegistration  = `INSERT INTO users (login, password, cookie) VALUES ($1, $2, $3) ON CONFLICT(login) DO NOTHING`
	dbAuthorization = `SELECT cookie FROM users WHERE login = $1 AND password = $2`
	dbChangeCookie  = `UPDATE users SET cookie = NULL WHERE cookie = $1`
	dbSetCookie     = `UPDATE users SET cookie = $1 WHERE login = $2 AND password = $3`
	dbGetLogin      = `SELECT login FROM users WHERE cookie = $1`

	// Таблица заказов orders:
	dbAddOrder      = `INSERT INTO Orders (number, login, uploaded_at) VALUES ($1, $2, $3) ON CONFLICT(number) DO NOTHING`
	dbGerOrders     = `SELECT number, status, accrual, uploaded_at FROM orders WHERE login = $1`
	dbGetOrderLogin = `SELECT login FROM Orders WHERE number = $1`
)

func StartDB(c config.Config) (*DataBase, error) {
	db, err := sql.Open("postgres", c.DataBaseURI)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err = db.PingContext(ctx); err != nil {
		return nil, err
	}

	if _, err = db.Exec(dbCreateUsersTable); err != nil {
		return nil, err
	}

	if _, err = db.Exec(dbCreateOrdersTable); err != nil {
		return nil, err
	}

	if _, err = db.Exec(dbCreateWithDrawTable); err != nil {
		return nil, err
	}

	var errs errs
	errs.Used = errors.New("used")
	errs.Empty = errors.New("empty")
	errs.Duplicate = errors.New("duplicate")
	errs.NoAuthorization = errors.New("no authorization")
	errs.RegisterConflict = errors.New("register conflict")

	return &DataBase{DB: db, Err: errs}, nil
}

func (db *DataBase) Register(login, pass, cookie string) error {
	exec, err := db.DB.Exec(dbRegistration, login, pass, cookie)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key value violates unique constraint \"users_cookie_key\"") {
			return db.Err.Duplicate
		}

		return err
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
			return db.Err.Empty
		}

		if !strings.Contains(err.Error(), "name \"cookie\": converting NULL to string is unsupported") {
			return err
		}
	}

	if cookieDB != cookie {
		if _, err := db.DB.Exec(dbChangeCookie, cookie); err != nil {
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

	exec, err := db.DB.Exec(dbAddOrder, order, login, time.Now().String())
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

	rows, err := db.DB.Query(dbGerOrders, login)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
	}

	var orders []Order
	for rows.Next() {
		var order Order
		var accrual sql.NullInt64
		if err = rows.Scan(&order.Number, &order.Status, &accrual, &order.UploadedAt); err != nil {
			if !errors.Is(err, sql.ErrNoRows) {
				return nil, err
			}

			if accrual.Valid {
				order.Accrual = int(accrual.Int64)
			}
		}

		orders = append(orders, order)
	}

	if rows.Err() != nil {
		return nil, err
	}

	return orders, nil
}
