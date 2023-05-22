package handlers

import (
	"github.com/chazari-x/yandex-pr-diplom/internal/app/config"
	"github.com/chazari-x/yandex-pr-diplom/internal/app/database"
	"github.com/chazari-x/yandex-pr-diplom/internal/app/worker"
)

type Controller struct {
	c      config.Config
	db     *database.DataBase
	worker chan worker.OrderStr
}

func NewController(c config.Config, db *database.DataBase, w chan worker.OrderStr) *Controller {
	return &Controller{c: c, db: db, worker: w}
}
