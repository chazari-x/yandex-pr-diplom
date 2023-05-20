package database

import (
	"context"
	"log"
	"reflect"
	"testing"
	"time"

	"github.com/chazari-x/yandex-pr-diplom/internal/app/config"
)

func TestCheckOrderNumber(t *testing.T) {
	tests := []struct {
		name string
		args int
		want bool
	}{
		{
			name: "",
			args: 01,
			want: false,
		},
		{
			name: "",
			args: 49927398716,
			want: true,
		},
		{
			name: "",
			args: 1234567812345670,
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := checkOrderNumber(tt.args); got != tt.want {
				t.Errorf("checkOrderNumber() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOrders(t *testing.T) {
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

	addOrder(t, db)

	getNotCheckedOrders(t, db)

	updateOrder(t, db)

	getOrders(t, db)

	getBalance(t, db)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, err = db.DB.ExecContext(ctx, `DROP TABLE users, orders, withdraw;`)
	if err != nil {
		log.Print(err)
		return
	}
}

func addOrder(t *testing.T, db *DataBase) {
	type addOrderArgs struct {
		login string
		order int
	}
	addOrder := []struct {
		name    string
		args    addOrderArgs
		wantErr bool
	}{
		{
			name: "",
			args: addOrderArgs{
				login: "username",
				order: 351243,
			},
			wantErr: true,
		},
		{
			name: "",
			args: addOrderArgs{
				login: "username",
				order: 49927398716,
			},
			wantErr: false,
		},
		{
			name: "",
			args: addOrderArgs{
				login: "username",
				order: 1234567812345670,
			},
			wantErr: false,
		},
	}
	for _, tt := range addOrder {
		t.Run(tt.name, func(t *testing.T) {
			if err := db.AddOrder(tt.args.login, tt.args.order); (err != nil) != tt.wantErr {
				t.Errorf("AddOrder() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func getNotCheckedOrders(t *testing.T, db *DataBase) {
	getNotCheckedOrders := []struct {
		name    string
		want    []string
		wantErr bool
	}{
		{
			name:    "",
			want:    []string{"49927398716", "1234567812345670"},
			wantErr: false,
		},
	}
	for _, tt := range getNotCheckedOrders {
		t.Run(tt.name, func(t *testing.T) {
			got, err := db.GetNotCheckedOrders()
			if (err != nil) != tt.wantErr {
				t.Errorf("GetNotCheckedOrders() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetNotCheckedOrders() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func updateOrder(t *testing.T, db *DataBase) {
	type updateOrderStr struct {
		number  string
		status  string
		accrual float64
	}
	updateOrder := []struct {
		name    string
		args    updateOrderStr
		wantErr bool
	}{
		{
			name: "",
			args: updateOrderStr{
				number:  "1234567812345670",
				status:  "PROCESSED",
				accrual: 535.31,
			},
			wantErr: false,
		},
	}
	for _, tt := range updateOrder {
		t.Run(tt.name, func(t *testing.T) {
			if err := db.UpdateOrder(tt.args.number, tt.args.status, tt.args.accrual); (err != nil) != tt.wantErr {
				t.Errorf("UpdateOrder() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func getOrders(t *testing.T, db *DataBase) {
	getOrders := []struct {
		name    string
		login   string
		want    []Order
		wantErr bool
	}{
		{
			name:  "",
			login: "username",
			want: []Order{
				{
					Number:     "49927398716",
					Status:     "NEW",
					UploadedAt: time.Now().Format(time.RFC3339),
				},
				{
					Number:     "1234567812345670",
					Status:     "PROCESSED",
					Accrual:    535.31,
					UploadedAt: time.Now().Format(time.RFC3339),
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range getOrders {
		t.Run(tt.name, func(t *testing.T) {
			got, err := db.GetOrders(tt.login)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetOrders() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetOrders() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func getBalance(t *testing.T, db *DataBase) {
	tests := []struct {
		name    string
		login   string
		want    User
		wantErr bool
	}{
		{
			name:  "",
			login: "username",
			want: User{
				Login:    "username",
				Current:  535.31,
				WithDraw: 0,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := db.GetBalance(tt.login)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetBalance() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetBalance() got = %v, want %v", got, tt.want)
			}
		})
	}
}
