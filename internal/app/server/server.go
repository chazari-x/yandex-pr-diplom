package server

import (
	"net/http"

	"github.com/chazari-x/yandex-pr-diplom/internal/app/config"
	"github.com/chazari-x/yandex-pr-diplom/internal/app/database"
	"github.com/chazari-x/yandex-pr-diplom/internal/app/handlers"
	"github.com/go-chi/chi/v5"
)

func StartServer() error {
	conf, err := config.GetConfig()
	if err != nil {
		return err
	}

	db, err := database.StartDB(conf)
	if err != nil {
		return err
	}

	c := handlers.NewController(conf, db)

	r := chi.NewRouter()

	r.Post(conf.AccrualSystemAddress+"/api/user/register", c.PostRegister)
	//регистрация пользователя

	r.Post(conf.AccrualSystemAddress+"/api/user/login", c.PostLogin)
	//аутентификация пользователя

	r.Post(conf.AccrualSystemAddress+"/api/user/orders", c.PostOrders)
	//загрузка пользователем номера заказа для расчета

	r.Get(conf.AccrualSystemAddress+"/api/user/orders", c.GetOrders)
	//получение списка загруженные пользователем номеров заказов, статусов их обработки и информации о начислениях

	r.Get(conf.AccrualSystemAddress+"/api/user/balance", c.GetBalance)
	//получение текущего баланса счета баллов лояльности пользователя

	r.Post(conf.AccrualSystemAddress+"/api/user/balance/withdraw", c.PostWithDraw)
	//запрос на списание баллов с накопительного счета в счет оплаты нового заказа

	r.Get(conf.AccrualSystemAddress+"/api/user/withdrawals", c.GetWithDrawAls)
	//получение информации о выводе средств накопительного счета пользователем

	r.Get(conf.AccrualSystemAddress+"/api/orders/{number}", c.GetOrdersNumber)
	//взаимодействие с системой расчёта начислений баллов лояльности

	return http.ListenAndServe(conf.RunAddress, handlers.MiddlewaresConveyor(r))
}
