package database

import (
	"context"
	"log"
	"testing"
	"time"

	"github.com/chazari-x/yandex-pr-diplom/internal/app/config"
)

type user struct {
	login  string
	pass   string
	cookie string
}

func TestUsers(t *testing.T) {
	db, err := StartDB(config.Config{DataBaseURI: "postgres://postgres:postgrespw@localhost:32768?sslmode=disable"})
	if err != nil {
		log.Print(err)
		return
	}

	defer func() {
		_ = db.DB.Close()
		log.Print("db closed")
	}()

	register(t, db)

	login(t, db)

	authentication(t, db)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, err = db.DB.ExecContext(ctx, `DROP TABLE users, orders, withdraw;`)
	if err != nil {
		log.Print(err)
		return
	}
}

func register(t *testing.T, db *DataBase) {
	log.Print("тест регистрации")

	tests := []struct {
		name    string
		args    user
		wantErr bool
	}{
		{
			name: "Пользователь 1",
			args: user{
				login:  "username1",
				pass:   "password",
				cookie: "0",
			},
			wantErr: false,
		},
		{
			name: "Пользователь 2",
			args: user{
				login:  "username2",
				pass:   "password",
				cookie: "0",
			},
			wantErr: false,
		},
		{
			name: "Пользователь 3",
			args: user{
				login:  "username3",
				pass:   "password",
				cookie: "4",
			},
			wantErr: false,
		},
		{
			name: "Пользователь 1",
			args: user{
				login:  "username1",
				pass:   "password1",
				cookie: "0",
			},
			wantErr: true,
		},
		{
			name: "Пользователь 5",
			args: user{
				login:  "username5",
				pass:   "password",
				cookie: "1",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run("Register: "+tt.name, func(t *testing.T) {
			if err := db.Register(tt.args.login, tt.args.pass, tt.args.cookie); (err != nil) != tt.wantErr {
				t.Errorf("Register() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func login(t *testing.T, db *DataBase) {
	log.Print("тест авторизации")

	tests := []struct {
		name    string
		args    user
		wantErr bool
	}{
		{
			name: "Пользователь 1",
			args: user{
				login:  "username1",
				pass:   "password",
				cookie: "9",
			},
			wantErr: false,
		},
		{
			name: "Пользователь 1",
			args: user{
				login:  "username1",
				pass:   "password2",
				cookie: "4",
			},
			wantErr: true,
		},
		{
			name: "Пользователь 3",
			args: user{
				login:  "username3",
				pass:   "password",
				cookie: "1",
			},
			wantErr: false,
		},
		{
			name: "Пользователь 1",
			args: user{
				login:  "username1",
				pass:   "password1",
				cookie: "9",
			},
			wantErr: true,
		},
		{
			name: "Пользователь 5",
			args: user{
				login:  "username5",
				pass:   "password",
				cookie: "4",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run("Login: "+tt.name, func(t *testing.T) {
			if err := db.Login(tt.args.login, tt.args.pass, tt.args.cookie); (err != nil) != tt.wantErr {
				t.Errorf("Login() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func authentication(t *testing.T, db *DataBase) {
	log.Print("тест аутентификации")

	tests := []struct {
		name    string
		cookie  string
		want    string
		wantErr bool
	}{
		{
			name:    "Пользователь 1",
			cookie:  "9",
			want:    "username1",
			wantErr: false,
		},
		{
			name:    "Пользователь 2",
			cookie:  "0",
			want:    "",
			wantErr: false,
		},
		{
			name:    "Пользователь 3",
			cookie:  "1",
			want:    "username3",
			wantErr: false,
		},
		{
			name:    "Пользователь 5",
			cookie:  "4",
			want:    "username5",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run("Authentication: "+tt.name, func(t *testing.T) {
			got, err := db.Authentication(tt.cookie)
			if (err != nil) != tt.wantErr {
				t.Errorf("Authentication() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Authentication() got = %v, want %v", got, tt.want)
			}
		})
	}
}
