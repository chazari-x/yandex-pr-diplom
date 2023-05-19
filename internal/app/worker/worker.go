package worker

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/chazari-x/yandex-pr-diplom/internal/app/config"
	"github.com/chazari-x/yandex-pr-diplom/internal/app/database"
)

type Controller struct {
	c  config.Config
	db *database.DataBase
}

type orderStr struct {
	Number  string  `json:"order"`
	Status  string  `json:"status"`
	Accrual float64 `json:"accrual"`
}

var InputCh = make(chan orderStr)

func StartWorker(conf config.Config, db *database.DataBase) error {
	orders, err := db.GetNotCheckedOrders()
	if err != nil {
		return err
	}

	go func(orders []string) {
		for _, order := range orders {
			InputCh <- orderStr{
				Number: order,
			}
		}
	}(orders)

	c := &Controller{c: conf, db: db}
	c.newWorker()

	return nil
}

func AddOrder(number string) {
	go func(number string) {
		InputCh <- orderStr{Number: number, Status: "NEW"}
	}(number)
}

func (c *Controller) newWorker() {
	go func() {
		log.Print("starting goroutine")

		defer func() {
			c.newWorker()
			if x := recover(); x != nil {
				log.Print("run time panic: ", x)
			}
		}()

		for {
			for o := range InputCh {
				resp, err := http.Get(c.c.AccrualSystemAddress + "/api/orders/" + o.Number)
				if err != nil {
					go func(o orderStr) {
						InputCh <- o
					}(o)
					log.Printf("go number: %s, err: %s", o.Number, err.Error())
					resp.Body.Close()
					continue
				}

				b, err := io.ReadAll(resp.Body)
				if err != nil {
					go func(o orderStr) {
						InputCh <- o
					}(o)
					log.Printf("go number: %s, err: %s", o.Number, err.Error())
					resp.Body.Close()
					continue
				}

				resp.Body.Close()

				switch resp.StatusCode {
				case http.StatusOK:
					var order orderStr
					err = json.Unmarshal(b, &order)
					if err != nil {
						go func(o orderStr) {
							InputCh <- o
						}(o)
						log.Printf("go number: %s, err: %s", o.Number, err.Error())
						continue
					}

					order.Number = o.Number

					switch order.Status {
					case "PROCESSING":
						log.Printf("go number: %s, status: %s", order.Number, order.Status)
						go func(o, order orderStr) {
							if o.Status != order.Status {
								err := c.db.UpdateOrder(order.Number, order.Status, order.Accrual)
								if err != nil {
									log.Printf("go number: %s, err: %s", order.Number, err.Error())
									return
								}
							}
							InputCh <- order
						}(o, order)
					case "INVALID", "PROCESSED":
						log.Printf("go number: %s, status: %s, accrual: %g", order.Number, order.Status, order.Accrual)
						go func(o orderStr, order orderStr) {
							if o.Status != order.Status {
								err := c.db.UpdateOrder(order.Number, order.Status, order.Accrual)
								if err != nil {
									InputCh <- order
									log.Printf("go number: %s, err: %s", o.Number, err.Error())
									return
								}
							}
						}(o, order)
					default:
						log.Printf("go number: %s, status: %s", o.Number, order.Status)
						go func(o orderStr) {
							InputCh <- o
						}(o)
					}
				case http.StatusTooManyRequests:
					log.Printf("go number: %s, status: %s", o.Number, resp.Status)
					go func(o orderStr) {
						InputCh <- o
					}(o)
					atoi, err := strconv.Atoi(resp.Header.Get("Retry-After"))
					if err != nil {
						log.Printf("go number: %s, err: %s", o.Number, err.Error())
						time.Sleep(time.Second * 15)
					} else {
						time.Sleep(time.Second * time.Duration(atoi))
					}
				case http.StatusInternalServerError:
					log.Printf("go number: %s, status: %s", o.Number, resp.Status)
					go func(o orderStr) {
						InputCh <- o
					}(o)
				case http.StatusNoContent:
					log.Printf("go number: %s, status: %s", o.Number, resp.Status)
					go func(o orderStr) {
						if o.Status != "PROCESSING" {
							err := c.db.UpdateOrder(o.Number, "PROCESSING", 0)
							if err != nil {
								log.Printf("go number: %s, err: %s", o.Number, err.Error())
								return
							}
							o.Status = "PROCESSING"
						}
						InputCh <- o
					}(o)
				default:
					log.Printf("go number: %s, status: %s", o.Number, resp.Status)
					go func(o orderStr) {
						InputCh <- o
					}(o)
				}
			}
		}
	}()
}
