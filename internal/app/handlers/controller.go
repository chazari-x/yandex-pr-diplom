package handlers

import (
	"github.com/chazari-x/yandex-pr-diplom/internal/app/config"
	"github.com/chazari-x/yandex-pr-diplom/internal/app/database"
)

type Controller struct {
	c  config.Config
	db *database.DataBase
}

func NewController(c config.Config, db *database.DataBase) *Controller {
	return &Controller{c: c, db: db}
}
