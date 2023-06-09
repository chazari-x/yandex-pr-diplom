package server

import (
	"log"
	"net/http"

	"github.com/chazari-x/yandex-pr-diplom/internal/app/config"
	"github.com/chazari-x/yandex-pr-diplom/internal/app/database"
	"github.com/chazari-x/yandex-pr-diplom/internal/app/handlers"
	"github.com/chazari-x/yandex-pr-diplom/internal/app/worker"
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

	defer func() {
		_ = db.DB.Close()
		log.Print("DB closed")
	}()

	w, err := worker.StartWorker(conf, db)
	if err != nil {
		return err
	}

	c := handlers.NewController(conf, db, w)

	r := chi.NewRouter()

	r.Post("/api/user/register", c.PostRegister)
	//регистрация пользователя

	r.Post("/api/user/login", c.PostLogin)
	//аутентификация пользователя

	r.Post("/api/user/orders", c.PostOrders)
	//загрузка пользователем номера заказа для расчета

	r.Get("/api/user/orders", c.GetOrders)
	//получение списка загруженные пользователем номеров заказов, статусов их обработки и информации о начислениях

	r.Get("/api/user/balance", c.GetBalance)
	//получение текущего баланса счета баллов лояльности пользователя

	r.Post("/api/user/balance/withdraw", c.PostWithDraw)
	//запрос на списание баллов с накопительного счета в счет оплаты нового заказа

	r.Get("/api/user/withdrawals", c.GetWithDrawAls)
	//получение информации о выводе средств накопительного счета пользователем

	return http.ListenAndServe(conf.RunAddress, c.MiddlewaresConveyor(r))
}
