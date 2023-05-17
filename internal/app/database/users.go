package database

import (
	"database/sql"
	"errors"
	"strings"
)

type User struct {
	UserID   string  `json:"user_id,omitempty"`
	Login    string  `json:"login,omitempty"`
	Password string  `json:"password,omitempty"`
	Cookie   string  `json:"cookie,omitempty"`
	Current  float64 `json:"current"`   // (сумма из orders) минус (сумма из withdraw)
	WithDraw float64 `json:"withdrawn"` // Сумма из withdraw
}

var (
	// Таблица пользователей users:
	dbRegistration  = `INSERT INTO users (login, password, cookie) VALUES ($1, $2, $3) ON CONFLICT(login) DO NOTHING`
	dbAuthorization = `SELECT cookie FROM users WHERE login = $1 AND password = $2`
	dbDellCookie    = `UPDATE users SET cookie = NULL WHERE cookie = $1`
	dbSetCookie     = `UPDATE users SET cookie = $1 WHERE login = $2 AND password = $3`
	dbGetLogin      = `SELECT login FROM users WHERE cookie = $1`
	dbGetBalance    = `SELECT login, 
						GREATEST(0, (SELECT SUM(accrual) FROM orders WHERE login = $1 GROUP BY login)) -
						GREATEST(0, (SELECT SUM(sum) FROM withdraw WHERE login = $1 GROUP BY login)),
						(SELECT SUM(sum) FROM withdraw WHERE login = $1 GROUP BY login) 
						FROM users WHERE login = $1`
)

func (db *DataBase) Register(login, pass, cookie string) error {
	if _, err := db.DB.Exec(dbDellCookie, cookie); err != nil {
		return err
	}

	exec, err := db.DB.Exec(dbRegistration, login, pass, cookie)
	if err != nil {
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
