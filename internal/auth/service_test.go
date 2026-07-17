package auth

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type fakeStore struct {
	initialized  bool
	users        map[string]User
	sessions     map[string]Session
	registration RegistrationParams
}

func newFakeStore() *fakeStore {
	return &fakeStore{users: make(map[string]User), sessions: make(map[string]Session)}
}

func (f *fakeStore) Initialized(context.Context) (bool, error) { return f.initialized, nil }
func (f *fakeStore) Bootstrap(_ context.Context, params BootstrapParams) error {
	if f.initialized {
		return ErrAlreadyInitialized
	}
	f.initialized = true
	f.users[params.User.Email] = params.User
	f.sessions[params.Session.TokenHash] = Session{ID: params.Session.ID, User: params.User, ExpiresAt: params.Session.ExpiresAt}
	return nil
}
func (f *fakeStore) UserByEmail(_ context.Context, email string) (User, error) {
	user, ok := f.users[email]
	if !ok {
		return User{}, errors.New("not found")
	}
	return user, nil
}
func (f *fakeStore) UserByUsername(_ context.Context, username string) (User, error) {
	for _, user := range f.users {
		if strings.EqualFold(user.Username, username) {
			return user, nil
		}
	}
	return User{}, errors.New("not found")
}
func (f *fakeStore) UserBySessionHash(_ context.Context, hash string, _ time.Time) (Session, error) {
	session, ok := f.sessions[hash]
	if !ok {
		return Session{}, errors.New("not found")
	}
	return session, nil
}
func (f *fakeStore) CreateSession(_ context.Context, input SessionInput) error {
	for _, user := range f.users {
		if user.ID == input.UserID {
			f.sessions[input.TokenHash] = Session{ID: input.ID, User: user, ExpiresAt: input.ExpiresAt}
			return nil
		}
	}
	return errors.New("user not found")
}
func (f *fakeStore) DeleteSessionByHash(_ context.Context, hash string) error {
	delete(f.sessions, hash)
	return nil
}
func (f *fakeStore) InvitationByHash(_ context.Context, _ string, now time.Time) (InvitationInfo, error) {
	return InvitationInfo{InviterName: "admin", ExpiresAt: now.Add(time.Hour)}, nil
}
func (f *fakeStore) RegisterWithInvitation(_ context.Context, params RegistrationParams, _ time.Time) error {
	f.registration = params
	f.users[params.User.Email] = params.User
	f.sessions[params.Session.TokenHash] = Session{ID: params.Session.ID, User: params.User, ExpiresAt: params.Session.ExpiresAt}
	return nil
}
func (f *fakeStore) RegistrationMode(_ context.Context) (string, error) { return "invite", nil }
func (f *fakeStore) RegisterOpen(_ context.Context, params RegistrationParams, _ time.Time) error {
	f.registration = params
	f.users[params.User.Email] = params.User
	f.sessions[params.Session.TokenHash] = Session{ID: params.Session.ID, User: params.User, ExpiresAt: params.Session.ExpiresAt}
	return nil
}

func TestBootstrapThenAuthenticate(t *testing.T) {
	store := newFakeStore()
	service := NewService(store, "01234567890123456789012345678901", time.Hour)
	service.now = func() time.Time { return time.Date(2026, 7, 16, 0, 0, 0, 0, time.UTC) }

	session, token, err := service.Bootstrap(context.Background(), "01234567890123456789012345678901", BootstrapInput{
		Username: "admin", Email: "ADMIN@example.com", Password: "strong password", InstanceName: "nav.ax", PublicBaseURL: "https://nav.ax/",
	})
	if err != nil {
		t.Fatal(err)
	}
	if session.User.Email != "admin@example.com" || token == "" || !store.initialized {
		t.Fatalf("unexpected bootstrap result: %+v", session)
	}
	authenticated, err := service.Authenticate(context.Background(), token)
	if err != nil || authenticated.User.ID != session.User.ID {
		t.Fatalf("Authenticate() = %+v, %v", authenticated, err)
	}
}

func TestLoginRejectsWrongPasswordAndDisabledUser(t *testing.T) {
	store := newFakeStore()
	service := NewService(store, "01234567890123456789012345678901", time.Hour)
	_, _, err := service.Bootstrap(context.Background(), "01234567890123456789012345678901", BootstrapInput{
		Username: "admin", Email: "admin@example.com", Password: "strong password", InstanceName: "nav.ax", PublicBaseURL: "https://nav.ax",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := service.Login(context.Background(), "admin@example.com", "wrong", "test"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("wrong password error = %v", err)
	}
	user := store.users["admin@example.com"]
	user.Status = "disabled"
	store.users[user.Email] = user
	if _, _, err := service.Login(context.Background(), user.Email, "strong password", "test"); !errors.Is(err, ErrAccountDisabled) {
		t.Fatalf("disabled user error = %v", err)
	}
}

func TestLoginAcceptsUsername(t *testing.T) {
	store := newFakeStore()
	service := NewService(store, "01234567890123456789012345678901", time.Hour)
	if _, _, err := service.Bootstrap(context.Background(), "01234567890123456789012345678901", BootstrapInput{
		Username: "admin", Email: "admin@example.com", Password: "strong password", InstanceName: "nav.ax", PublicBaseURL: "https://nav.ax",
	}); err != nil {
		t.Fatal(err)
	}
	session, token, err := service.Login(context.Background(), "Admin", "strong password", "test")
	if err != nil || token == "" || session.User.Username != "admin" {
		t.Fatalf("username login = %+v token=%q err=%v", session, token, err)
	}
}

func TestLoginThrottleBlocksThenRecovers(t *testing.T) {
	store := newFakeStore()
	service := NewService(store, "01234567890123456789012345678901", time.Hour)
	clock := time.Date(2026, 7, 16, 0, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return clock }
	if _, _, err := service.Bootstrap(context.Background(), "01234567890123456789012345678901", BootstrapInput{
		Username: "admin", Email: "admin@example.com", Password: "strong password", InstanceName: "nav.ax", PublicBaseURL: "https://nav.ax",
	}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < service.throttle.threshold; i++ {
		if _, _, err := service.Login(context.Background(), "admin@example.com", "wrong", "test"); !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("attempt %d error = %v", i, err)
		}
	}
	// Even the correct password is refused while the account is locked.
	_, _, err := service.Login(context.Background(), "admin@example.com", "strong password", "test")
	if !errors.Is(err, ErrTooManyAttempts) {
		t.Fatalf("locked login error = %v, want ErrTooManyAttempts", err)
	}
	var throttled *ThrottledError
	if !errors.As(err, &throttled) || throttled.RetryAfter <= 0 {
		t.Fatalf("throttled error carries no retry-after: %v", err)
	}
	// After the window elapses, the correct password succeeds and clears state.
	clock = clock.Add(service.throttle.baseLock + time.Second)
	if _, _, err := service.Login(context.Background(), "admin@example.com", "strong password", "test"); err != nil {
		t.Fatalf("login after lock window = %v", err)
	}
}

func TestRegisterUsesInvitationHashAndCreatesPersonalPage(t *testing.T) {
	store := newFakeStore()
	service := NewService(store, "01234567890123456789012345678901", time.Hour)
	_, token, err := service.Register(context.Background(), "invite-token", RegisterInput{
		Username: "alice", Email: "Alice@Example.com", Password: "strong password", Device: "browser",
	})
	if err != nil {
		t.Fatal(err)
	}
	if token == "" || store.registration.InvitationHash == "invite-token" || store.registration.PageID == "" || store.registration.Slug != "alice" {
		t.Fatalf("unexpected registration params: %+v", store.registration)
	}
}

func TestRegisterRejectsWeakOrMalformedIdentity(t *testing.T) {
	service := NewService(newFakeStore(), "01234567890123456789012345678901", time.Hour)
	for _, input := range []RegisterInput{
		{Username: "a", Email: "a@example.com", Password: "strong password"},
		{Username: "alice", Email: "not-an-email", Password: "strong password"},
		{Username: "alice", Email: "a@example.com", Password: "short"},
	} {
		if _, _, err := service.Register(context.Background(), "invite-token", input); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("Register(%+v) error = %v", input, err)
		}
	}
}
