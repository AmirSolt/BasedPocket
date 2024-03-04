package extension

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/getsentry/sentry-go"
	"github.com/labstack/echo/v5"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/models"
	"github.com/stripe/stripe-go/v76"
	"github.com/stripe/stripe-go/v76/webhook"
)

func HandleStripeWebhook(app core.App, ctx echo.Context, env *Env) error {
	// ==================================================================
	// The signature check is pulled directly from Stripe and it's not tested
	req := ctx.Request()
	res := ctx.Response()

	const MaxBodyBytes = int64(65536)
	req.Body = http.MaxBytesReader(res.Writer, req.Body, MaxBodyBytes)
	payload, err := io.ReadAll(req.Body)
	if err != nil {
		eventId := sentry.CaptureException(err)
		ctx.String(http.StatusServiceUnavailable, fmt.Errorf("problem with request. EventID: %s", *eventId).Error())
		return err
	}
	endpointSecret := env.STRIPE_WEBHOOK_KEY
	event, err := webhook.ConstructEvent(payload, req.Header.Get("Stripe-Signature"), endpointSecret)
	if err != nil {
		eventId := sentry.CaptureException(err)
		ctx.String(http.StatusBadRequest, fmt.Errorf("error verifying webhook signature. EventID: %s", *eventId).Error())
		return err
	}
	// ==================================================================

	if err := handleStripeEvents(app, event); err != nil {
		ctx.String(http.StatusInternalServerError, err.Error())
		return err
	}

	res.Writer.WriteHeader(http.StatusOK)
	return nil
}

func handleStripeEvents(app core.App, event stripe.Event) error {

	if event.Type == "customer.created" {
		return handleCustomerCreatedEvent(app, event)
	}
	if event.Type == "customer.deleted" {
		return handleCustomerDeletedEvent(app, event)
	}
	if event.Type == "customer.subscription.created" {
		return handleSubscriptionCreatedEvent(app, event)
	}
	if event.Type == "customer.subscription.updated" {
		return handleSubscriptionUpdatedEvent(app, event)
	}
	if event.Type == "customer.subscription.deleted" {
		return handleSubscriptionDeletedEvent(app, event)
	}

	err := fmt.Errorf("unhandled stripe event type: %s\n", event.Type)
	eventId := sentry.CaptureException(err)
	return fmt.Errorf("unhandled stripe event type. EventID: %s", *eventId)
}

// ===============================================================================

func handleCustomerCreatedEvent(app core.App, event stripe.Event) error {
	stripeCustomer, err := getStripeCustomerFromObj(event.Data.Object)
	if err != nil {
		eventId := sentry.CaptureException(err)
		return fmt.Errorf("error handling stripe event. EventID: %s", *eventId)
	}

	user, err := app.Dao().FindFirstRecordByData("users", "email", stripeCustomer.Email)
	if err != nil {
		eventId := sentry.CaptureException(err)
		return fmt.Errorf("error handling stripe event. EventID: %s", *eventId)
	}

	customers, err := app.Dao().FindCollectionByNameOrId("customers")
	if err != nil {
		eventId := sentry.CaptureException(err)
		return fmt.Errorf("error handling stripe event. EventID: %s", *eventId)
	}
	customer := models.NewRecord(customers)
	customer.Set("user", user.Id)
	customer.Set("stripe_customer_id", stripeCustomer.ID)
	customer.Set("stripe_subscription_id", nil)
	customer.Set("tier", 0)
	if err := app.Dao().SaveRecord(customer); err != nil {
		eventId := sentry.CaptureException(err)
		return fmt.Errorf("error handling stripe event. EventID: %s", *eventId)
	}
	return nil
}

// ===============================================================================

func handleCustomerDeletedEvent(app core.App, event stripe.Event) error {
	stripeCustomer, err := getStripeCustomerFromObj(event.Data.Object)
	if err != nil {
		eventId := sentry.CaptureException(err)
		return fmt.Errorf("error handling stripe event. EventID: %s", *eventId)
	}

	customer, err := app.Dao().FindFirstRecordByData("customers", "stripe_customer_id", stripeCustomer.ID)
	if err != nil {
		eventId := sentry.CaptureException(err)
		return fmt.Errorf("error handling stripe event. EventID: %s", *eventId)
	}
	if err := app.Dao().DeleteRecord(customer); err != nil {
		return err
	}
	return nil
}

// ===============================================================================

func handleSubscriptionCreatedEvent(app core.App, event stripe.Event) error {
	stripeSubscription, err := getStripeSubscriptionFromObj(event.Data.Object)
	if err != nil {
		eventId := sentry.CaptureException(err)
		return fmt.Errorf("error handling stripe event. EventID: %s", *eventId)
	}

	tier, err := getSubscriptionTier(stripeSubscription)
	if err != nil {
		eventId := sentry.CaptureException(err)
		return fmt.Errorf("error handling stripe event. EventID: %s", *eventId)
	}

	customer, err := app.Dao().FindFirstRecordByData("customers", "stripe_customer_id", stripeSubscription.Customer)
	if err != nil {
		eventId := sentry.CaptureException(err)
		return fmt.Errorf("error handling stripe event. EventID: %s", *eventId)
	}

	customer.Set("stripe_subscription_id", stripeSubscription.ID)
	customer.Set("tier", tier)
	if err := app.Dao().SaveRecord(customer); err != nil {
		eventId := sentry.CaptureException(err)
		return fmt.Errorf("error handling stripe event. EventID: %s", *eventId)
	}
	return nil
}

// ===============================================================================

func handleSubscriptionUpdatedEvent(app core.App, event stripe.Event) error {
	stripeSubscription, err := getStripeSubscriptionFromObj(event.Data.Object)
	if err != nil {
		eventId := sentry.CaptureException(err)
		return fmt.Errorf("error handling stripe event. EventID: %s", *eventId)
	}

	tier, err := getSubscriptionTier(stripeSubscription)
	if err != nil {
		eventId := sentry.CaptureException(err)
		return fmt.Errorf("error handling stripe event. EventID: %s", *eventId)
	}

	customer, err := app.Dao().FindFirstRecordByData("customers", "stripe_subscription_id", stripeSubscription.ID)
	if err != nil {
		eventId := sentry.CaptureException(err)
		return fmt.Errorf("error handling stripe event. EventID: %s", *eventId)
	}

	customer.Set("tier", tier)
	if err := app.Dao().SaveRecord(customer); err != nil {
		eventId := sentry.CaptureException(err)
		return fmt.Errorf("error handling stripe event. EventID: %s", *eventId)
	}
	return nil
}

// ===============================================================================

func handleSubscriptionDeletedEvent(app core.App, event stripe.Event) error {
	stripeSubscription, err := getStripeSubscriptionFromObj(event.Data.Object)
	if err != nil {
		eventId := sentry.CaptureException(err)
		return fmt.Errorf("error handling stripe event. EventID: %s", *eventId)
	}

	customer, err := app.Dao().FindFirstRecordByData("customers", "stripe_customer_id", stripeSubscription.Customer)
	if err != nil {
		eventId := sentry.CaptureException(err)
		return fmt.Errorf("error handling stripe event. EventID: %s", *eventId)
	}

	customer.Set("stripe_subscription_id", nil)
	customer.Set("tier", 0)
	if err := app.Dao().SaveRecord(customer); err != nil {
		eventId := sentry.CaptureException(err)
		return fmt.Errorf("error handling stripe event. EventID: %s", *eventId)
	}
	return nil
}

// ===============================================================================
// ===============================================================================
// ===============================================================================

func getStripeCustomerFromObj(object map[string]interface{}) (*stripe.Customer, error) {
	jsonCustomer, err := json.Marshal(object)
	if err != nil {
		eventId := sentry.CaptureException(err)
		return nil, fmt.Errorf("error handling stripe event. EventID: %s", *eventId)
	}
	var stripeCustomer *stripe.Customer
	err = json.Unmarshal(jsonCustomer, &stripeCustomer)
	if stripeCustomer == nil || err != nil {
		eventId := sentry.CaptureException(err)
		return nil, fmt.Errorf("error handling stripe event. EventID: %s", *eventId)
	}
	return stripeCustomer, nil
}

func getStripeCheckoutSessionFromObj(object map[string]interface{}) (*stripe.CheckoutSession, error) {
	jsonCustomer, err := json.Marshal(object)
	if err != nil {
		eventId := sentry.CaptureException(err)
		return nil, fmt.Errorf("error handling stripe event. EventID: %s", *eventId)
	}
	var stripeStruct *stripe.CheckoutSession
	err = json.Unmarshal(jsonCustomer, &stripeStruct)
	if stripeStruct == nil || err != nil {
		eventId := sentry.CaptureException(err)
		return nil, fmt.Errorf("error handling stripe event. EventID: %s", *eventId)
	}
	return stripeStruct, nil
}

func getStripeSubscriptionFromObj(object map[string]interface{}) (*stripe.Subscription, error) {
	jsonCustomer, err := json.Marshal(object)
	if err != nil {
		eventId := sentry.CaptureException(err)
		return nil, fmt.Errorf("error handling stripe event. EventID: %s", *eventId)
	}
	var stripeStruct *stripe.Subscription
	err = json.Unmarshal(jsonCustomer, &stripeStruct)
	if stripeStruct == nil || err != nil {
		eventId := sentry.CaptureException(err)
		return nil, fmt.Errorf("error handling stripe event. EventID: %s", *eventId)
	}
	return stripeStruct, nil
}

func getSubscriptionTier(subsc *stripe.Subscription) (int, error) {
	if subsc == nil {
		return 0, nil
	}
	subscTierStr := subsc.Items.Data[0].Price.Metadata["tier"]
	subscTierInt, errTier := strconv.Atoi(subscTierStr)
	if errTier != nil {
		eventId := sentry.CaptureException(errTier)
		return 0, fmt.Errorf("failed to convert tier (%v)", eventId)
	}
	return subscTierInt, nil
}
