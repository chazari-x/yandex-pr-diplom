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
	DB    *sql.DB
	Users Users
}

type Users struct {
	UID      string `json:"uid"`
	Login    string `json:"login"`
	Password string `json:"password"`
}

type Orders struct {
	UID        string `json:"uid"`
	Number     string `json:"number"`
	Status     string `json:"status"`
	Accrual    int    `json:"accrual"`
	UploadedAt string `json:"uploaded_at"`
}

type WithDraw struct {
	UID         string `json:"uid"`
	Order       string `json:"order_"`
	Sum         int    `json:"sum"`
	ProcessedAt string `json:"processed_at"`
}

var (
	dbCreateUsersTable = `CREATE TABLE IF NOT EXISTS users (
							id				SERIAL  PRIMARY KEY NOT NULL,
							login			VARCHAR UNIQUE		NOT NULL, 
							password		VARCHAR 			NOT NULL, 
							cookie			VARCHAR UNIQUE		NULL,
							current			INTEGER 			NOT NULL	DEFAULT 0, 
							withdraw		INTEGER 			NOT NULL	DEFAULT 0)`

	dbCreateOrdersTable = `CREATE TABLE IF NOT EXISTS orders (
							login 			VARCHAR PRIMARY KEY NOT NULL, 
							number 			VARCHAR UNIQUE 		NOT NULL,
							status 			VARCHAR 			NOT NULL, 
							accrual 		INTEGER 			NOT NULL,
							uploaded_at 	VARCHAR				NOT NULL)`

	dbCreateWithDrawTable = `CREATE TABLE IF NOT EXISTS withdraw (
							login 			VARCHAR	PRIMARY KEY NOT NULL,
							order_ 			VARCHAR 			NOT NULL,
							sum 			INTEGER 			NOT NULL,
							processed_at	VARCHAR 			NOT NULL)`

	dbAuthorization = `SELECT cookie FROM users WHERE login = $1 AND password = $2`
	dbChangeCookie  = `UPDATE users SET cookie = NULL WHERE cookie = $1`
	dbSetCookie     = `UPDATE users SET cookie = $1 WHERE login = $2 AND password = $3`

	dbRegistration = `INSERT INTO users (login, password, cookie) VALUES ($1, $2, $3) ON CONFLICT(login) DO NOTHING`
)

var (
	ErrRegisterConflict = errors.New("register conflict")
	ErrEmpty            = errors.New("empty")
	ErrDuplicateIdent   = errors.New("duplicate cookie")
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

	_, err = db.Exec(dbCreateUsersTable)
	if err != nil {
		return nil, err
	}

	_, err = db.Exec(dbCreateOrdersTable)
	if err != nil {
		return nil, err
	}

	_, err = db.Exec(dbCreateWithDrawTable)
	if err != nil {
		return nil, err
	}

	return &DataBase{DB: db}, nil
}

func (db *DataBase) Register(login, pass, cookie string) error {
	exec, err := db.DB.Exec(dbRegistration, login, pass, cookie)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key value violates unique constraint \"users_cookie_key\"") {
			return ErrDuplicateIdent
		}

		return err
	}

	affected, err := exec.RowsAffected()
	if err != nil {
		return err
	}

	if affected == 0 {
		return ErrRegisterConflict
	}

	return nil
}

func (db *DataBase) Login(login, pass, cookie string) error {
	var cookieDB string

	err := db.DB.QueryRow(dbAuthorization, login, pass).Scan(&cookieDB)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrEmpty
		}

		if !strings.Contains(err.Error(), "name \"cookie\": converting NULL to string is unsupported") {
			return err
		}
	}

	if cookieDB != cookie {
		_, err = db.DB.Exec(dbChangeCookie, cookie)
		if err != nil {
			return err
		}

		_, err = db.DB.Exec(dbSetCookie, cookie, login, pass)
		if err != nil {
			return err
		}
	}

	return nil
}
