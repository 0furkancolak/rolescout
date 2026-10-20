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
	HasUsers(ctx context.Context) (bool, error)
	ListUsers(ctx context.Context) ([]domain.User, error)
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

// newAccount decodes and validates a sign-up body and hashes the password.
func (h *handlers) newAccount(c fiber.Ctx) (database.RegisterParams, error) {
	var req registerRequest
	if err := decodeJSON(c, &req); err != nil {
		return database.RegisterParams{}, err
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
		return database.RegisterParams{}, err
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		return database.RegisterParams{}, err
	}
	workspaceName := "Kişisel"
	if locale == "en" {
		workspaceName = "Personal"
	}
	return database.RegisterParams{Email: email, DisplayName: name, Locale: locale, PasswordHash: hash, WorkspaceName: workspaceName}, nil
}

// register is the public sign-up: open for the first account only (like
// Grafana's first-run setup), or for everyone with ALLOW_REGISTRATION=true.
func (h *handlers) register(c fiber.Ctx) error {
	p, err := h.newAccount(c)
	if err != nil {
		return err
	}
	p.AllowWhenUsersExist = h.AuthConfig.AllowRegistration
	user, _, err := h.Auth.Register(c.Context(), p)
	if err != nil {
		return err
	}
	if err := h.startSession(c, user.ID); err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(dataResponse[userDTO]{Data: toUser(user)})
}

// setupStatus tells the sign-in page whether to offer "create account".
func (h *handlers) setupStatus(c fiber.Ctx) error {
	has, err := h.Auth.HasUsers(c.Context())
	if err != nil {
		return err
	}
	return c.JSON(dataResponse[setupDTO]{Data: setupDTO{RegistrationOpen: !has || h.AuthConfig.AllowRegistration}})
}

// adminListUsers lists every account on this installation.
func (h *handlers) adminListUsers(c fiber.Ctx) error {
	users, err := h.Auth.ListUsers(c.Context())
	if err != nil {
		return err
	}
	out := make([]userDTO, len(users))
	for i, u := range users {
		out[i] = toUser(u)
	}
	return c.JSON(dataResponse[[]userDTO]{Data: out})
}

// adminCreateUser lets an administrator add an account. It does not sign the
// new user in, and the account is a regular (non-admin) one.
func (h *handlers) adminCreateUser(c fiber.Ctx) error {
	p, err := h.newAccount(c)
	if err != nil {
		return err
	}
	p.AllowWhenUsersExist = true
	user, _, err := h.Auth.Register(c.Context(), p)
	if err != nil {
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
