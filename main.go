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

	env := extension.LoadEnv()
	extension.LoadLogging(env)

	app := pocketbase.New()

	app.OnBeforeServe().Add(func(e *core.ServeEvent) error {
		extension.CreateCustomersCollection(e.App)
		return nil
	})

	app.OnBeforeServe().Add(func(e *core.ServeEvent) error {
		e.Router.AddRoute(echo.Route{
			Method: http.MethodPost,
			Path:   "/webhooks/stripe",
			Handler: func(c echo.Context) error {
				return extension.HandleStripeWebhook(e.App, c, env)
			},
			Middlewares: []echo.MiddlewareFunc{
				apis.ActivityLogger(e.App),
			},
		})

		return nil
	})

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
