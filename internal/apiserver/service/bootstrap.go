package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/CrazyTomatoOo/model-go/internal/apiserver/store"
	"github.com/CrazyTomatoOo/model-go/internal/password"
)

// ErrAdminsExist aborts the bootstrap run when an admin account already
// exists (login design spec §9: idempotent refusal).
var ErrAdminsExist = errors.New("admin accounts already exist")

type adminCreator interface {
	HasAnyAdmin(ctx context.Context) (bool, error)
	CreateAdmins(ctx context.Context, admins []store.NewAdmin) error
}

// BootstrapService seeds the three initial admin accounts (spec §8/§9).
type BootstrapService struct {
	users adminCreator
}

func NewBootstrapService(users adminCreator) *BootstrapService {
	return &BootstrapService{users: users}
}

// seedAccounts are the three initial admins: role and username (spec §8).
var seedAccounts = []struct {
	username string
	role     string
}{
	{"sysadmin", store.RoleSysadmin},
	{"secadmin", store.RoleSecadmin},
	{"auditadmin", store.RoleAuditadmin},
}

// Bootstrap creates the sysadmin/secadmin/auditadmin accounts from the
// given plaintext passwords, hashing each and enforcing the spec §6 policy.
// It refuses when any admin already exists and persists nothing when any
// password fails validation.
func (s *BootstrapService) Bootstrap(ctx context.Context, sysadminPwd, secadminPwd, auditadminPwd string) error {
	exists, err := s.users.HasAnyAdmin(ctx)
	if err != nil {
		return err
	}
	if exists {
		return ErrAdminsExist
	}

	passwords := []string{sysadminPwd, secadminPwd, auditadminPwd}
	admins := make([]store.NewAdmin, 0, len(seedAccounts))
	for i, seed := range seedAccounts {
		if err := password.Validate(passwords[i], seed.username); err != nil {
			return fmt.Errorf("%s password: %w", seed.username, err)
		}
		hash, err := password.Hash(passwords[i])
		if err != nil {
			return fmt.Errorf("hash %s password: %w", seed.username, err)
		}
		admins = append(admins, store.NewAdmin{Username: seed.username, PasswordHash: hash, Role: seed.role})
	}
	return s.users.CreateAdmins(ctx, admins)
}
