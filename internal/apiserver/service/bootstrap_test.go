package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/CrazyTomatoOo/model-go/internal/apiserver/service"
	"github.com/CrazyTomatoOo/model-go/internal/apiserver/store"
	"github.com/CrazyTomatoOo/model-go/internal/password"
)

type stubAdminCreator struct {
	exists    bool
	existsErr error
	created   []store.NewAdmin
	createErr error
}

func (s *stubAdminCreator) HasAnyAdmin(context.Context) (bool, error) {
	return s.exists, s.existsErr
}

func (s *stubAdminCreator) CreateAdmins(_ context.Context, admins []store.NewAdmin) error {
	s.created = admins
	return s.createErr
}

const pwd = "V3ry-Long-Passw0rd!"

func TestBootstrapCreatesThreeAdmins(t *testing.T) {
	creator := &stubAdminCreator{}
	err := service.NewBootstrapService(creator).Bootstrap(context.Background(), pwd, pwd, pwd)
	require.NoError(t, err)

	require.Len(t, creator.created, 3)
	want := []struct {
		username, role string
	}{
		{"sysadmin", store.RoleSysadmin},
		{"secadmin", store.RoleSecadmin},
		{"auditadmin", store.RoleAuditadmin},
	}
	for i, w := range want {
		require.Equal(t, w.username, creator.created[i].Username)
		require.Equal(t, w.role, creator.created[i].Role)
		match, err := password.Verify(pwd, creator.created[i].PasswordHash)
		require.NoError(t, err, "hash must verify")
		require.True(t, match, "hash must be of the seeded password")
	}
}

func TestBootstrapRefusesWhenAdminExists(t *testing.T) {
	creator := &stubAdminCreator{exists: true}
	err := service.NewBootstrapService(creator).Bootstrap(context.Background(), pwd, pwd, pwd)
	require.ErrorIs(t, err, service.ErrAdminsExist)
	require.Empty(t, creator.created, "must not create admins when one already exists")
}

func TestBootstrapHasAnyAdminErrorPropagates(t *testing.T) {
	creator := &stubAdminCreator{existsErr: errors.New("db down")}
	err := service.NewBootstrapService(creator).Bootstrap(context.Background(), pwd, pwd, pwd)
	require.ErrorContains(t, err, "db down")
	require.Empty(t, creator.created)
}

func TestBootstrapWeakPasswordPersistsNothing(t *testing.T) {
	creator := &stubAdminCreator{}
	err := service.NewBootstrapService(creator).Bootstrap(context.Background(), "short", pwd, pwd)
	require.ErrorIs(t, err, password.ErrTooShort)
	require.Empty(t, creator.created, "no admin may be created when a later password is weak")
}

func TestBootstrapPasswordContainingUsernameRejected(t *testing.T) {
	creator := &stubAdminCreator{}
	err := service.NewBootstrapService(creator).Bootstrap(context.Background(), pwd, "secadmin-12345678", pwd)
	require.ErrorIs(t, err, password.ErrContainsUsername)
	require.Empty(t, creator.created)
}

func TestBootstrapCreateErrorPropagates(t *testing.T) {
	creator := &stubAdminCreator{createErr: errors.New("tx failed")}
	err := service.NewBootstrapService(creator).Bootstrap(context.Background(), pwd, pwd, pwd)
	require.ErrorContains(t, err, "tx failed")
}
