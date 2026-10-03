package authrepo

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/auth"
)

func TestGoogleRepositoryIdentityConcurrencyAndNullPassword(t *testing.T) {
	ctx, repository, pool := integrationRepository(t)
	identity := auth.GoogleIdentity{Subject: testUUID(), Email: testEmail()}
	var wait sync.WaitGroup
	ids := make(chan string, 6)
	for index := 0; index < 6; index++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			user, pending, err := repository.ResolveGoogleIdentity(ctx, identity, auth.User{ID: testUUID(), Email: identity.Email, Role: auth.RoleUser})
			if err != nil || pending {
				t.Errorf("resolve: %v pending=%v", err, pending)
				return
			}
			ids <- user.ID
		}()
	}
	wait.Wait()
	close(ids)
	var userID string
	for id := range ids {
		if userID != "" && userID != id {
			t.Fatal("duplicate users for Google subject")
		}
		userID = id
	}
	if userID == "" {
		t.Fatal("no user")
	}
	t.Cleanup(func() { pool.Exec(context.Background(), "DELETE FROM users WHERE id=$1", userID) })
	user, err := repository.FindUserByID(ctx, userID)
	if err != nil || user.PasswordHash != "" {
		t.Fatalf("nullable scan: %+v %v", user, err)
	}
	user, err = repository.FindUserByEmail(ctx, identity.Email)
	if err != nil || user.ID != userID {
		t.Fatalf("nullable email scan: %+v %v", user, err)
	}
	identity.Email = testEmail()
	changed, pending, err := repository.ResolveGoogleIdentity(ctx, identity, auth.User{ID: testUUID(), Email: identity.Email, Role: auth.RoleUser})
	if err != nil || pending || changed.ID != userID || changed.Email != user.Email {
		t.Fatalf("subject lookup changed ownership: %+v %v", changed, err)
	}
	if err := repository.CreateRefreshSession(ctx, auth.RefreshSession{ID: testUUID(), UserID: userID, TokenHash: tokenHash(testUUID()), ExpiresAt: time.Now().Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
}

func TestGoogleRepositoryStateSingleUseAndExpiry(t *testing.T) {
	ctx, repository, pool := integrationRepository(t)
	stateHash, browserHash := tokenHash(testUUID()), tokenHash(testUUID())
	t.Cleanup(func() {
		pool.Exec(context.Background(), "DELETE FROM auth_google_transactions WHERE state_hash=$1", stateHash)
	})
	transaction := auth.GoogleTransaction{StateHash: stateHash, BrowserHash: browserHash, Nonce: "test-nonce", Verifier: "test-verifier", ReturnTo: "/reports", ExpiresAt: time.Now().Add(time.Minute)}
	if err := repository.CreateGoogleTransaction(ctx, transaction); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.ConsumeGoogleTransaction(ctx, stateHash, tokenHash("different-browser"), time.Now()); !errors.Is(err, auth.ErrGoogleState) {
		t.Fatalf("wrong browser: %v", err)
	}
	if _, err := repository.ConsumeGoogleTransaction(ctx, stateHash, browserHash, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.ConsumeGoogleTransaction(ctx, stateHash, browserHash, time.Now()); !errors.Is(err, auth.ErrGoogleState) {
		t.Fatalf("replay: %v", err)
	}
	transaction.ExpiresAt = time.Now().Add(-time.Second)
	if err := repository.CreateGoogleTransaction(ctx, transaction); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.ConsumeGoogleTransaction(ctx, stateHash, browserHash, time.Now()); !errors.Is(err, auth.ErrGoogleState) {
		t.Fatalf("expired: %v", err)
	}
}

func TestGoogleMigrationRefusesDestructiveRollback(t *testing.T) {
	ctx, repository, pool := integrationRepository(t)
	user, _, err := repository.ResolveGoogleIdentity(ctx, auth.GoogleIdentity{Subject: testUUID(), Email: testEmail()}, auth.User{ID: testUUID(), Role: auth.RoleUser})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pool.Exec(context.Background(), "DELETE FROM users WHERE id=$1", user.ID) })
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "migrations", "000013_google_auth.down.sql"))
	if err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, string(data)); err == nil {
		t.Fatal("rollback silently removed Google-only authentication")
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.FindUserByID(ctx, user.ID); err != nil {
		t.Fatal("rollback damaged account")
	}
}

func TestGoogleRepositoryCollisionLinkAndAttemptLimit(t *testing.T) {
	ctx, repository, pool := integrationRepository(t)
	user := createTestUser(t, ctx, repository, pool, auth.RoleUser)
	identity := auth.GoogleIdentity{Subject: testUUID(), Email: user.Email}
	resolved, needsLink, err := repository.ResolveGoogleIdentity(ctx, identity, auth.User{ID: testUUID(), Email: user.Email, Role: auth.RoleUser})
	if err != nil || !needsLink || resolved.ID != user.ID {
		t.Fatalf("collision: %+v %v", resolved, err)
	}
	pending := auth.GooglePending{TokenHash: tokenHash(testUUID()), UserID: user.ID, Subject: identity.Subject, ReturnTo: "/reports", ExpiresAt: time.Now().Add(time.Minute)}
	if err := repository.CreateGooglePending(ctx, pending); err != nil {
		t.Fatal(err)
	}
	for attempt := 0; attempt < 5; attempt++ {
		_, _, err := repository.CompleteGoogleLink(ctx, pending.TokenHash, time.Now(), func(auth.User) bool { return false })
		if err == nil {
			t.Fatal("wrong proof accepted")
		}
	}
	if _, _, err := repository.CompleteGoogleLink(ctx, pending.TokenHash, time.Now(), func(auth.User) bool { return true }); !errors.Is(err, auth.ErrGoogleAttempts) {
		t.Fatalf("attempt limit: %v", err)
	}
	pending.TokenHash = tokenHash(testUUID())
	if err := repository.CreateGooglePending(ctx, pending); err != nil {
		t.Fatal(err)
	}
	linked, returnTo, err := repository.CompleteGoogleLink(ctx, pending.TokenHash, time.Now(), func(candidate auth.User) bool { return candidate.ID == user.ID })
	if err != nil || linked.ID != user.ID || returnTo != "/reports" {
		t.Fatalf("link: %+v %v", linked, err)
	}
	if _, _, err := repository.CompleteGoogleLink(ctx, pending.TokenHash, time.Now(), func(auth.User) bool { return true }); !errors.Is(err, auth.ErrGoogleLink) {
		t.Fatalf("link replay: %v", err)
	}
}
