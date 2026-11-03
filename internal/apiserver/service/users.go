package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"

	"github.com/CrazyTomatoOo/model-go/internal/apiserver/store"
	"github.com/CrazyTomatoOo/model-go/internal/password"
)

// Error kinds for the sysadmin user-management endpoints (login design
// spec §4.5).
const (
	KindValidation = "VALIDATION_ERROR"
	KindNotFound   = "NOT_FOUND"
	KindConflict   = "CONFLICT"
)

// User-management outcomes (spec §4.2): a uniform 404 and a 409 that never
// echoes the conflicting username (no existence/field oracle).
var (
	ErrNotFound      = &Error{Kind: KindNotFound, Message: "用户不存在"}
	ErrUsernameTaken = &Error{Kind: KindConflict, Message: "用户名已存在"}
)

// Validation outcomes carry the offending field for §4.5 field detail.
var (
	errUsernameRequired = &Error{Kind: KindValidation, Field: "username", Message: "用户名不能为空"}
	errPasswordRequired = &Error{Kind: KindValidation, Field: "password", Message: "密码不能为空"}
	errRoleInvalid      = &Error{Kind: KindValidation, Field: "role", Message: "角色无效"}
	errPasswordReused   = &Error{Kind: KindValidation, Field: "password", Message: "新密码不能与当前或最近使用的密码相同"}
)

// allowedRoles are the roles creatable through the user-management API
// (spec §4.2: user/sysadmin/secadmin/auditadmin).
var allowedRoles = []string{store.RoleUser, store.RoleSysadmin, store.RoleSecadmin, store.RoleAuditadmin}

// The user-management store seams, narrowed so the service is unit-testable
// with stubs.
type (
	UserAdminStore interface {
		ByID(ctx context.Context, id int64) (store.User, error)
		Create(ctx context.Context, username, passwordHash, role string, createdBy int64) (int64, error)
		PasswordHistory(ctx context.Context, userID int64) ([]string, error)
		ResetPassword(ctx context.Context, userID int64, newHash string) error
		Disable(ctx context.Context, userID int64) error
	}
	SessionKicker interface {
		DeleteByUserID(ctx context.Context, userID int64) (int, error)
	}
)

// CreateUserInput is the client-owned subset of a user creation (spec §4.2:
// status/must_change_password/created_by are server-forced, never accepted
// from the request body).
type CreateUserInput struct {
	Username string
	Password string
	Role     string
}

// CreatedUser is the creation response (spec §4.2: id plus the always
// pending status).
type CreatedUser struct {
	ID     int64
	Status string
}

// UsersService implements the sysadmin account-management operations
// (spec §4.2): create, password reset, and disable.
type UsersService struct {
	users    UserAdminStore
	sessions SessionKicker
	audits   AuditStore
	logger   *slog.Logger
}

func NewUsersService(users UserAdminStore, sessions SessionKicker, audits AuditStore, logger *slog.Logger) *UsersService {
	return &UsersService{users: users, sessions: sessions, audits: audits, logger: logger}
}

// Create validates the input, hashes the password, and stores a pending
// account created by the acting admin (spec §4.2). It returns ErrUsernameTaken
// on a collision and audits account.create.
func (s *UsersService) Create(ctx context.Context, actor store.User, in CreateUserInput, ip string) (CreatedUser, error) {
	if in.Username == "" {
		return CreatedUser{}, errUsernameRequired
	}
	if in.Password == "" {
		return CreatedUser{}, errPasswordRequired
	}
	if !slices.Contains(allowedRoles, in.Role) {
		return CreatedUser{}, errRoleInvalid
	}
	if err := password.Validate(in.Password, in.Username); err != nil {
		return CreatedUser{}, policyError(err)
	}
	hash, err := password.Hash(in.Password)
	if err != nil {
		return CreatedUser{}, err
	}
	id, err := s.users.Create(ctx, in.Username, hash, in.Role, actor.ID)
	if err != nil {
		if errors.Is(err, store.ErrUsernameTaken) {
			return CreatedUser{}, ErrUsernameTaken
		}
		return CreatedUser{}, err
	}
	s.auditAccount(ctx, actor, "account.create", id, ip)
	return CreatedUser{ID: id, Status: store.StatusPending}, nil
}

// ResetPassword runs the spec §4.2 full reset: policy validation against the
// username, history-reuse check against the current hash plus the last 5
// stored hashes, store update, session kick, and the auth.password_reset
// audit row.
func (s *UsersService) ResetPassword(ctx context.Context, actor store.User, userID int64, newPassword, ip string) error {
	user, err := s.users.ByID(ctx, userID)
	if err != nil {
		if errors.Is(err, store.ErrUserNotFound) {
			// Equalize with the reuse-check path (~one argon2 verify), so a
			// never-existing id 404s in the same time as one that vanished
			// mid-flight (spec §4.5: no existence leak via timing).
			_, _ = password.Verify(newPassword, dummyHash)
			return ErrNotFound
		}
		return err
	}
	if err := password.Validate(newPassword, user.Username); err != nil {
		return policyError(err)
	}
	history, err := s.users.PasswordHistory(ctx, userID)
	if err != nil {
		return err
	}
	reused, err := password.CheckReuse(ctx, newPassword, user.PasswordHash, history)
	if err != nil {
		return err
	}
	if reused {
		return errPasswordReused
	}
	hash, err := password.Hash(newPassword)
	if err != nil {
		return err
	}
	if err := s.users.ResetPassword(ctx, userID, hash); err != nil {
		if errors.Is(err, store.ErrUserNotFound) {
			return ErrNotFound
		}
		return err
	}
	if _, err := s.sessions.DeleteByUserID(ctx, userID); err != nil {
		return err
	}
	s.auditAccount(ctx, actor, "auth.password_reset", userID, ip)
	return nil
}

// Disable sets the user's status to disabled and kicks all of their
// sessions, then audits account.disable (spec §4.2).
func (s *UsersService) Disable(ctx context.Context, actor store.User, userID int64, ip string) error {
	if err := s.users.Disable(ctx, userID); err != nil {
		if errors.Is(err, store.ErrUserNotFound) {
			return ErrNotFound
		}
		return err
	}
	if _, err := s.sessions.DeleteByUserID(ctx, userID); err != nil {
		return err
	}
	s.auditAccount(ctx, actor, "account.disable", userID, ip)
	return nil
}

// auditAccount writes one account-management audit row (best-effort, like
// the auth audits: the outcome is never masked by a failed audit write).
func (s *UsersService) auditAccount(ctx context.Context, actor store.User, action string, userID int64, ip string) {
	if err := s.audits.Log(ctx, store.AuditEntry{
		ActorID:       &actor.ID,
		ActorUsername: actor.Username,
		ActorRole:     actor.Role,
		Action:        action,
		Target:        fmt.Sprintf("users:%d", userID),
		IP:            ip,
	}); err != nil {
		s.logger.Error("audit "+action, "user_id", userID, "error", err)
	}
}

// policyError maps a password-policy failure (spec §6) to a typed validation
// error carrying the offending field.
func policyError(err error) *Error {
	var tooShort *password.TooShortError
	switch {
	case errors.As(err, &tooShort):
		return &Error{Kind: KindValidation, Field: "password",
			Message: fmt.Sprintf("密码长度不足，至少需要 %d 位", tooShort.MinLength)}
	case errors.Is(err, password.ErrContainsUsername):
		return &Error{Kind: KindValidation, Field: "password", Message: "密码不能包含用户名"}
	case errors.Is(err, password.ErrTooCommon):
		return &Error{Kind: KindValidation, Field: "password", Message: "密码过于常见，请更换"}
	default:
		return &Error{Kind: KindValidation, Field: "password", Message: "密码不符合安全策略"}
	}
}
