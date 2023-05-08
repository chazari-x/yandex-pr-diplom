package main

import (
	"log"

	"github.com/chazari-x/yandex-pr-diplom/internal/app/server"
)

func main() {
	log.Print(server.StartServer())
}
