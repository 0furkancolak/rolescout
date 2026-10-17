package httptransport

import (
	"context"
	"errors"
	"net/mail"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"

	"example.com/rolescout/api/internal/auth"
	"example.com/rolescout/api/internal/domain"
	"example.com/rolescout/api/internal/infrastructure/database"
)

const (
	sessionCookie    = "rolescout_session"
	userLocal        = "user"
	maxEmailLength   = 254
	maxNameLength    = 100
	maxWorkspaceName = 100
)

type AuthStore interface {
	Register(ctx context.Context, p database.RegisterParams) (domain.User, domain.Workspace, error)
	Credentials(ctx context.Context, email string) (domain.User, string, error)
	CreateSession(ctx context.Context, tokenHash []byte, userID string, expiresAt time.Time) error
	UserBySession(ctx context.Context, tokenHash []byte) (domain.User, error)
	DeleteSession(ctx context.Context, tokenHash []byte) error
}

// AuthConfig holds the session and registration settings.
type AuthConfig struct {
	SessionTTL        time.Duration
	CookieSecure      bool
	AllowRegistration bool
}

var (
	errUnauthenticated    = errors.New("unauthenticated")
	errInvalidCredentials = errors.New("invalid credentials")
)

// requireUser authenticates the session cookie and stores the user in Locals.
func (h *handlers) requireUser(c fiber.Ctx) error {
	token := c.Cookies(sessionCookie)
	if token == "" {
		return errUnauthenticated
	}
	user, err := h.Auth.UserBySession(c.Context(), auth.HashSessionToken(token))
	if errors.Is(err, domain.ErrNotFound) {
		return errUnauthenticated
	}
	if err != nil {
		return err
	}
	c.Locals(userLocal, user)
	return c.Next()
}

func currentUser(c fiber.Ctx) (domain.User, bool) {
	u, ok := c.Locals(userLocal).(domain.User)
	return u, ok
}

// actorID identifies the authenticated user for created_by fields.
func actorID(c fiber.Ctx) *string {
	u, ok := currentUser(c)
	if !ok {
		return nil
	}
	return &u.ID
}

func (h *handlers) startSession(c fiber.Ctx, userID string) error {
	token, hash, err := auth.NewSessionToken()
	if err != nil {
		return err
	}
	expires := time.Now().Add(h.AuthConfig.SessionTTL)
	if err := h.Auth.CreateSession(c.Context(), hash, userID, expires); err != nil {
		return err
	}
	c.Cookie(&fiber.Cookie{
		Name: sessionCookie, Value: token, Path: "/", Expires: expires,
		MaxAge: int(h.AuthConfig.SessionTTL.Seconds()), Secure: h.AuthConfig.CookieSecure,
		HTTPOnly: true, SameSite: fiber.CookieSameSiteLaxMode,
	})
	return nil
}

func (h *handlers) clearSessionCookie(c fiber.Ctx) {
	c.Cookie(&fiber.Cookie{
		Name: sessionCookie, Value: "", Path: "/", Expires: time.Unix(0, 0), MaxAge: -1,
		Secure: h.AuthConfig.CookieSecure, HTTPOnly: true, SameSite: fiber.CookieSameSiteLaxMode,
	})
}

type registerRequest struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
	Locale      string `json:"locale"`
}

func validateEmail(v *domain.ValidationError, raw string) string {
	email := strings.ToLower(strings.TrimSpace(raw))
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email || len(email) > maxEmailLength {
		v.Add("email", "must be a valid email address")
	}
	return email
}

func (h *handlers) register(c fiber.Ctx) error {
	var req registerRequest
	if err := decodeJSON(c, &req); err != nil {
		return err
	}
	var v domain.ValidationError
	email := validateEmail(&v, req.Email)
	if msg := auth.ValidatePassword(req.Password); msg != "" {
		v.Add("password", "%s", msg)
	}
	name := requiredText(&v, "display_name", req.DisplayName, maxNameLength)
	locale := req.Locale
	if locale == "" {
		locale = "tr"
	}
	if locale != "tr" && locale != "en" {
		v.Add("locale", "must be tr or en")
	}
	if err := v.Err(); err != nil {
		return err
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		return err
	}
	workspaceName := "Kişisel"
	if locale == "en" {
		workspaceName = "Personal"
	}
	user, _, err := h.Auth.Register(c.Context(), database.RegisterParams{
		Email: email, DisplayName: name, Locale: locale, PasswordHash: hash,
		WorkspaceName: workspaceName, AllowWhenUsersExist: h.AuthConfig.AllowRegistration,
	})
	if err != nil {
		return err
	}
	if err := h.startSession(c, user.ID); err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(dataResponse[userDTO]{Data: toUser(user)})
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *handlers) login(c fiber.Ctx) error {
	var req loginRequest
	if err := decodeJSON(c, &req); err != nil {
		return err
	}
	if req.Email == "" || req.Password == "" || len(req.Password) > 4*auth.MaxPasswordLength {
		return errInvalidCredentials
	}

	user, hash, err := h.Auth.Credentials(c.Context(), req.Email)
	if errors.Is(err, domain.ErrNotFound) {
		auth.SpendVerificationTime(req.Password)
		return errInvalidCredentials
	}
	if err != nil {
		return err
	}
	ok, err := auth.VerifyPassword(req.Password, hash)
	if err != nil {
		return err
	}
	if !ok {
		return errInvalidCredentials
	}

	// A new session per login prevents session fixation.
	if err := h.startSession(c, user.ID); err != nil {
		return err
	}
	return c.JSON(dataResponse[userDTO]{Data: toUser(user)})
}

// logout revokes the current session if there is one and always clears the cookie.
func (h *handlers) logout(c fiber.Ctx) error {
	if token := c.Cookies(sessionCookie); token != "" {
		if err := h.Auth.DeleteSession(c.Context(), auth.HashSessionToken(token)); err != nil {
			return err
		}
	}
	h.clearSessionCookie(c)
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *handlers) me(c fiber.Ctx) error {
	user, _ := currentUser(c)
	return c.JSON(dataResponse[userDTO]{Data: toUser(user)})
}
