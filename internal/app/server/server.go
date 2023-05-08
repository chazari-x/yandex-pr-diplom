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

	r.Post("/api/user/register", c.Register)
	//регистрация пользователя

	r.Post("/api/user/login", c.Login)
	//аутентификация пользователя

	//r.Post("/api/user/orders", )
	//загрузка пользователем номера заказа для расчета

	//r.Get	("/api/user/orders", )
	//получение списка загруженные пользователем номеров заказов, статусов их обработки и информации о начислениях

	//r.Get	("/api/user/balance", )
	//получение текущего баланса счета баллов лояльности пользователя

	//r.Post("/api/user/balance/withdraw", )
	//запрос на списание баллов с накопительного счета в счет оплаты нового заказа

	//r.Get	("/api/user/withdrawals", )
	//получение информации о выводе средств накопительного счета пользователем

	return http.ListenAndServe(conf.RunAddress, handlers.MiddlewaresConveyor(r))
}
