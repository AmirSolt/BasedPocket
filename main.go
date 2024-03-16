package main

import (
	"basedpocket/extension"
	"log"
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

// go run main.go serve
//

func main() {

	env := base.LoadEnv()
	base.LoadLogging(env)
	app := pocketbase.New()

	cmodels.LoadModels(app, env)
	payment.LoadPayment(app, env)

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
