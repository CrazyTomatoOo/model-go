package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/CrazyTomatoOo/model-go/internal/apiserver/store"
	"github.com/CrazyTomatoOo/model-go/internal/password"
)

// Password-change outcomes distinct from the login ones (spec §6).
var (
	ErrWrongOldPassword = &Error{Kind: KindValidationError, Field: "old_password", Message: "旧密码不正确"}
	ErrPasswordReused   = &Error{Kind: KindValidationError, Field: "new_password", Message: "新密码不得与当前密码或最近 5 次历史密码相同"}
)

// ChangePassword implements the spec §6 flow: verify the old password,
// validate the new one against the policy and the last-5 history, swap the
// hash (recording the old one in password_history), clear
// must_change_password, kick the user's other sessions, and audit.
func (s *AuthService) ChangePassword(ctx context.Context, sid string, user store.User, oldPassword, newPassword, ip string) error {
	match, err := password.Verify(oldPassword, user.PasswordHash)
	if err != nil {
		return err
	}
	if !match {
		return ErrWrongOldPassword
	}
	if err := password.Validate(newPassword, user.Username); err != nil {
		return passwordPolicyError(err)
	}
	history, err := s.users.HistoryHashes(ctx, user.ID)
	if err != nil {
		return err
	}
	reused, err := password.CheckReuse(ctx, newPassword, user.PasswordHash, history)
	if err != nil {
		return err
	}
	if reused {
		return ErrPasswordReused
	}
	newHash, err := password.Hash(newPassword)
	if err != nil {
		return err
	}
	if err := s.users.ChangePassword(ctx, user.ID, newHash, user.PasswordHash); err != nil {
		return err
	}
	s.kickOtherSessions(ctx, user.ID, sid)
	s.auditPasswordChange(ctx, user, ip)
	return nil
}

// passwordPolicyError maps a policy failure to a field-scoped validation
// error on new_password.
func passwordPolicyError(err error) *Error {
	var tooShort *password.TooShortError
	switch {
	case errors.As(err, &tooShort):
		return &Error{Kind: KindValidationError, Field: "new_password", Message: fmt.Sprintf("密码长度不足，至少需要 %d 位", tooShort.MinLength)}
	case errors.Is(err, password.ErrContainsUsername):
		return &Error{Kind: KindValidationError, Field: "new_password", Message: "密码不得与用户名相同或包含用户名"}
	case errors.Is(err, password.ErrTooCommon):
		return &Error{Kind: KindValidationError, Field: "new_password", Message: "密码过于常见，请更换新密码"}
	default:
		return &Error{Kind: KindValidationError, Field: "new_password", Message: "密码不符合密码策略要求"}
	}
}

// kickOtherSessions deletes every session of the user except the one that
// performed the change; failures are logged, never surfaced.
func (s *AuthService) kickOtherSessions(ctx context.Context, userID int64, keepSID string) {
	sids, err := s.sessions.ListByUser(ctx, userID)
	if err != nil {
		s.logger.Error("kick sessions after password change", "user_id", userID, "error", err)
		return
	}
	for _, sid := range sids {
		if sid == keepSID {
			continue
		}
		if err := s.sessions.Delete(ctx, sid); err != nil {
			s.logger.Error("kick session after password change", "user_id", userID, "sid", sid, "error", err)
		}
	}
}

// auditPasswordChange writes one auth.password_change row (best-effort).
func (s *AuthService) auditPasswordChange(ctx context.Context, user store.User, ip string) {
	if err := s.audits.Log(ctx, store.AuditEntry{
		ActorID:       &user.ID,
		ActorUsername: user.Username,
		ActorRole:     user.Role,
		Action:        "auth.password_change",
		Target:        fmt.Sprintf("users:%d", user.ID),
		IP:            ip,
	}); err != nil {
		s.logger.Error("audit password change", "user_id", user.ID, "error", err)
	}
}
