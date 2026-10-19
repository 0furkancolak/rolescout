package httptransport

import (
	"context"
	"encoding/json"

	"github.com/gofiber/fiber/v3"

	"example.com/rolescout/api/internal/domain"
)

// UserStateStore keeps the web app's per-user state (the job browser's last
// view, autofill details).
type UserStateStore interface {
	Get(ctx context.Context, userID, workspaceID, key string) (json.RawMessage, error)
	Put(ctx context.Context, userID, workspaceID, key string, value json.RawMessage) error
}

var userStateKeys = map[string]bool{"jobs_view": true, "autofill": true}

const maxUserStateBytes = 64 * 1024

func (h *handlers) stateKey(c fiber.Ctx) (string, error) {
	key := c.Params("key")
	if !userStateKeys[key] {
		return "", domain.ErrNotFound
	}
	return key, nil
}

func (h *handlers) getUserState(c fiber.Ctx) error {
	key, err := h.stateKey(c)
	if err != nil {
		return err
	}
	v, err := h.UserState.Get(c.Context(), *actorID(c), workspaceID(c), key)
	if err != nil {
		return err
	}
	return c.JSON(dataResponse[json.RawMessage]{Data: v})
}

func (h *handlers) putUserState(c fiber.Ctx) error {
	key, err := h.stateKey(c)
	if err != nil {
		return err
	}
	var v map[string]any
	if len(c.Body()) > maxUserStateBytes {
		return (&domain.ValidationError{}).Add("body", "must be at most %d bytes", maxUserStateBytes)
	}
	if err := decodeJSON(c, &v); err != nil {
		return err
	}
	raw, _ := json.Marshal(v)
	if err := h.UserState.Put(c.Context(), *actorID(c), workspaceID(c), key, raw); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}
