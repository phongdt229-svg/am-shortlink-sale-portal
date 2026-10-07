package scope

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"am-shortlink-portal/api/internal/domain"
)

func TestAdminUnrestricted(t *testing.T) {
	s := For(domain.Principal{Username: "root", Role: domain.RoleAdmin})
	f, err := s.Filter("owner", nil)
	require.NoError(t, err)
	require.Empty(t, f)

	f, err = s.Filter("owner", []string{"partner_a"})
	require.NoError(t, err)
	require.Equal(t, bson.D{{Key: "owner", Value: "partner_a"}}, f)
}

func TestUserOnlyOwnData(t *testing.T) {
	s := For(domain.Principal{Username: "partner_a", Role: domain.RoleUser})
	f, err := s.Filter("meta.owner", nil)
	require.NoError(t, err)
	require.Equal(t, bson.D{{Key: "meta.owner", Value: "partner_a"}}, f)

	_, err = s.Filter("owner", []string{"partner_b"})
	e, ok := domain.AsError(err)
	require.True(t, ok)
	require.Equal(t, http.StatusForbidden, e.Status)
}

func TestViewerAssignedAccounts(t *testing.T) {
	s := For(domain.Principal{Username: "v", Role: domain.RoleViewer, ViewerAccounts: []string{"b", "a", "b"}})
	require.Equal(t, []string{"a", "b"}, s.Accounts())
	require.False(t, s.Allows("v"), "viewer không tự động thấy tài khoản của chính mình")

	f, err := s.Filter("owner", nil)
	require.NoError(t, err)
	require.Equal(t, bson.D{{Key: "owner", Value: bson.D{{Key: "$in", Value: []string{"a", "b"}}}}}, f)

	_, err = s.Filter("owner", []string{"a", "c"})
	require.Error(t, err)
}

func TestViewerWithoutAccountsSeesNothing(t *testing.T) {
	s := For(domain.Principal{Username: "v", Role: domain.RoleViewer})
	f, err := s.Filter("owner", nil)
	require.NoError(t, err)
	require.Equal(t, bson.D{{Key: "owner", Value: bson.D{{Key: "$in", Value: []string{}}}}}, f)
}
