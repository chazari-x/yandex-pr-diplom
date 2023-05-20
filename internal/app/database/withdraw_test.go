package database

import (
	"context"
	"log"
	"reflect"
	"testing"
	"time"

	"github.com/chazari-x/yandex-pr-diplom/internal/app/config"
)

func TestWithDraw(t *testing.T) {
	db, err := StartDB(config.Config{DataBaseURI: "postgres://postgres:postgrespw@localhost:32768?sslmode=disable"})
	if err != nil {
		log.Print(err)
		return
	}

	defer func() {
		_ = db.DB.Close()
		log.Print("db closed")
	}()

	t.Run("Регистрация", func(t *testing.T) {
		if err := db.Register("username", "password", "0124"); (err != nil) != false {
			t.Errorf("Register() error = %v, wantErr %v", err, false)
		}
	})

	t.Run("Пополнение баланса", func(t *testing.T) {
		if err := db.AddOrder("username", 49927398716); (err != nil) != false {
			t.Errorf("AddOrder() error = %v, wantErr %v", err, false)
		}
	})

	t.Run("Подтверждение пополнения", func(t *testing.T) {
		if err := db.UpdateOrder("49927398716", "PROCESSED", 500); (err != nil) != false {
			t.Errorf("UpdateOrder() error = %v, wantErr %v", err, false)
		}
	})

	addWithDraw(t, db)

	getWithDraw(t, db)

	t.Run("Проверка баланса", func(t *testing.T) {
		got, err := db.GetBalance("username")
		if (err != nil) != false {
			t.Errorf("GetBalance() error = %v, wantErr %v", err, false)
			return
		}
		want := User{Login: "username", Current: 500 - 161, WithDraw: 161}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("GetBalance() got = %v, want %v", got, want)
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, err = db.DB.ExecContext(ctx, `DROP TABLE users, orders, withdraw;`)
	if err != nil {
		log.Print(err)
		return
	}
}

func addWithDraw(t *testing.T, db *DataBase) {
	type args struct {
		login string
		order string
		sum   float64
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "",
			args: args{
				login: "username",
				order: "1735735",
				sum:   161,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := db.AddWithDraw(tt.args.login, tt.args.order, tt.args.sum); (err != nil) != tt.wantErr {
				t.Errorf("AddWithDraw() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func getWithDraw(t *testing.T, db *DataBase) {
	tests := []struct {
		name    string
		login   string
		want    []WithDraw
		wantErr bool
	}{
		{
			name:  "",
			login: "username",
			want: []WithDraw{
				{
					OrderID:     "1735735",
					Sum:         161,
					ProcessedAt: time.Now().Format(time.RFC3339),
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := db.GetWithDraw(tt.login)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetWithDraw() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetWithDraw() got = %v, want %v", got, tt.want)
			}
		})
	}
}
