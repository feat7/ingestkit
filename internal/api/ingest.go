package api

import (
	"context"
	"github.com/feat7/ingestkit/internal/api/middleware"
	"github.com/feat7/ingestkit/internal/event"
	"github.com/gofiber/fiber/v2"
	"time"
)

func ingest(publisher EventPublisher, timeout time.Duration, batch bool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		raw := append([]byte(nil), c.Body()...)
		var inputs []event.Input
		var err error
		if batch {
			inputs, err = event.DecodeBatch(raw)
		} else {
			var in event.Input
			in, err = event.DecodeInput(raw)
			inputs = []event.Input{in}
		}
		if err != nil {
			status := 400
			if event.IsTooLarge(err) {
				status = 413
			}
			return middleware.SendError(c, status, "validation_failed", err.Error())
		}
		tenant, ok := c.Locals("tenant_id").(string)
		if !ok {
			return middleware.SendError(c, 500, "internal_error", "missing tenant identity")
		}
		received := time.Now().UTC()
		events := make([]event.Envelope, len(inputs))
		ids := make([]string, len(inputs))
		for i, in := range inputs {
			events[i], err = event.Accept(in, tenant, received)
			if err != nil {
				return middleware.SendError(c, 400, "validation_failed", "invalid event identity")
			}
			ids[i] = in.EventID.String()
		}
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		if err = publisher.Publish(ctx, events); err != nil {
			return middleware.SendError(c, 503, "unavailable", "delivery is uncertain; retry with the same event IDs")
		}
		return c.Status(202).JSON(fiber.Map{"status": "accepted", "event_ids": ids, "request_id": c.Locals("request_id")})
	}
}
