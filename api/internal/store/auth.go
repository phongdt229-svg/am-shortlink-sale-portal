package store

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"am-shortlink-portal/api/internal/auth"
)

// Store thoả auth.Store.
var _ auth.Store = (*Store)(nil)

type userDoc struct {
	Username       string   `bson:"username"`
	Email          string   `bson:"email"`
	PasswordHash   string   `bson:"password_hash"`
	Role           string   `bson:"role"`
	Active         *bool    `bson:"active"`
	PortalAccess   bool     `bson:"portal_access"`
	ViewerAccounts []string `bson:"viewer_accounts"`
}

func (s *Store) FindUser(ctx context.Context, username string) (*auth.UserRecord, error) {
	ctx, cancel := s.ctx(ctx)
	defer cancel()
	var d userDoc
	err := s.core(CollUsers).FindOne(ctx, bson.D{{Key: "username", Value: username}},
		options.FindOne().SetProjection(bson.D{
			{Key: "username", Value: 1}, {Key: "email", Value: 1}, {Key: "password_hash", Value: 1},
			{Key: "role", Value: 1}, {Key: "active", Value: 1}, {Key: "portal_access", Value: 1},
			{Key: "viewer_accounts", Value: 1},
		})).Decode(&d)
	if isNoDocs(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &auth.UserRecord{
		Username:       d.Username,
		Email:          d.Email,
		PasswordHash:   d.PasswordHash,
		Role:           d.Role,
		Active:         d.Active != nil && *d.Active, // thiếu trường → coi như không hoạt động
		PortalAccess:   d.PortalAccess,
		ViewerAccounts: d.ViewerAccounts,
	}, nil
}

type loginAttemptDoc struct {
	Username    string    `bson:"_id"`
	Failures    int       `bson:"failures"`
	LockedUntil time.Time `bson:"locked_until,omitempty"`
	UpdatedAt   time.Time `bson:"updated_at"`
}

func (s *Store) LockedUntil(ctx context.Context, username string) (time.Time, error) {
	ctx, cancel := s.ctx(ctx)
	defer cancel()
	var d loginAttemptDoc
	err := s.report(CollLoginAttempts).FindOne(ctx, bson.D{{Key: "_id", Value: username}}).Decode(&d)
	if isNoDocs(err) {
		return time.Time{}, nil
	}
	return d.LockedUntil, err
}

func (s *Store) RecordFailure(ctx context.Context, username string, now time.Time, max int, lockFor time.Duration) (time.Time, error) {
	ctx, cancel := s.ctx(ctx)
	defer cancel()
	coll := s.report(CollLoginAttempts)
	var d loginAttemptDoc
	err := coll.FindOneAndUpdate(ctx,
		bson.D{{Key: "_id", Value: username}},
		bson.D{
			{Key: "$inc", Value: bson.D{{Key: "failures", Value: 1}}},
			{Key: "$set", Value: bson.D{{Key: "updated_at", Value: now}}},
		},
		options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After),
	).Decode(&d)
	if err != nil {
		return time.Time{}, err
	}
	if d.Failures < max {
		return d.LockedUntil, nil
	}
	until := now.Add(lockFor)
	_, err = coll.UpdateOne(ctx, bson.D{{Key: "_id", Value: username}}, bson.D{{Key: "$set", Value: bson.D{
		{Key: "failures", Value: 0}, {Key: "locked_until", Value: until}, {Key: "updated_at", Value: now},
	}}})
	return until, err
}

func (s *Store) ResetFailures(ctx context.Context, username string) error {
	ctx, cancel := s.ctx(ctx)
	defer cancel()
	_, err := s.report(CollLoginAttempts).DeleteOne(ctx, bson.D{{Key: "_id", Value: username}})
	return err
}

type refreshDoc struct {
	Hash          string     `bson:"_id"`
	FamilyID      string     `bson:"family_id"`
	Username      string     `bson:"username"`
	CreatedAt     time.Time  `bson:"created_at"`
	ExpiresAt     time.Time  `bson:"expires_at"`
	RevokedAt     *time.Time `bson:"revoked_at,omitempty"`
	FamilyRevoked bool       `bson:"family_revoked,omitempty"`
	UserAgent     string     `bson:"user_agent,omitempty"`
	IP            string     `bson:"ip,omitempty"`
}

func (s *Store) InsertRefresh(ctx context.Context, r auth.RefreshRecord) error {
	ctx, cancel := s.ctx(ctx)
	defer cancel()
	_, err := s.report(CollRefreshTokens).InsertOne(ctx, refreshDoc(r))
	return err
}

func (s *Store) FindRefresh(ctx context.Context, hash string) (*auth.RefreshRecord, error) {
	ctx, cancel := s.ctx(ctx)
	defer cancel()
	var d refreshDoc
	err := s.report(CollRefreshTokens).FindOne(ctx, bson.D{{Key: "_id", Value: hash}}).Decode(&d)
	if isNoDocs(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	r := auth.RefreshRecord(d)
	return &r, nil
}

func (s *Store) RevokeRefresh(ctx context.Context, hash string, now time.Time) (bool, error) {
	ctx, cancel := s.ctx(ctx)
	defer cancel()
	res, err := s.report(CollRefreshTokens).UpdateOne(ctx,
		bson.D{{Key: "_id", Value: hash}, {Key: "revoked_at", Value: bson.D{{Key: "$exists", Value: false}}}},
		bson.D{{Key: "$set", Value: bson.D{{Key: "revoked_at", Value: now}}}})
	if err != nil {
		return false, err
	}
	return res.ModifiedCount == 1, nil
}

func (s *Store) RevokeFamily(ctx context.Context, familyID string, now time.Time) error {
	ctx, cancel := s.ctx(ctx)
	defer cancel()
	coll := s.report(CollRefreshTokens)
	if _, err := coll.UpdateMany(ctx,
		bson.D{{Key: "family_id", Value: familyID}},
		bson.D{{Key: "$set", Value: bson.D{{Key: "family_revoked", Value: true}}}}); err != nil {
		return err
	}
	_, err := coll.UpdateMany(ctx,
		bson.D{{Key: "family_id", Value: familyID}, {Key: "revoked_at", Value: bson.D{{Key: "$exists", Value: false}}}},
		bson.D{{Key: "$set", Value: bson.D{{Key: "revoked_at", Value: now}}}})
	return err
}
