//go:build !js

package sqlite

import (
	"context"
	"testing"

	"fiatjaf.com/nostr"
	"fiatjaf.com/nostr/nip11"
	_ "github.com/mattn/go-sqlite3"
)

func TestGetUser(t *testing.T) {
	ctx := context.Background()
	path := t.TempDir() + "/test.sqlite"
	db, err := testOpenDB(path)
	if err != nil {
		t.Fatal(err)
	}
	store, err := New(db)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	// Insert a test user metadata event
	pubkey, _ := nostr.PubKeyFromHex("0000000000000000000000000000000000000000000000000000000000000001")
	id, _ := nostr.IDFromHex("0000000000000000000000000000000000000000000000000000000000000001")
	event := &nostr.Event{
		ID:        id,
		PubKey:    pubkey,
		Kind:      0,
		CreatedAt: nostr.Timestamp(1000),
		Content:   `{"name":"Test User"}`,
	}

	_, err = store.Save(ctx, event)
	if err != nil {
		t.Fatal(err)
	}

	// Test getting the user - first manually insert to test the function
	_, err = store.DB.ExecContext(ctx, `
		INSERT INTO users (pubkey, name, about, picture, website, banner, bot, lud16, event_id, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, pubkey.String(), "Test User", "Test about", "https://example.com/pic.jpg", "https://example.com", nil, nil, "test@example.com", id.String(), 1000)
	if err != nil {
		t.Fatalf("Manual insert failed: %v", err)
	}

	user, err := store.GetUser(ctx, pubkey.String())
	if err != nil {
		t.Fatalf("GetUser failed: %v", err)
	}

	if user == nil {
		t.Fatal("expected user, got nil")
	}

	if user.PubKey != pubkey.String() {
		t.Errorf("expected pubkey %s, got %s", pubkey.String(), user.PubKey)
	}

	if !user.Name.Valid || user.Name.String != "Test User" {
		t.Errorf("expected name 'Test User', got %v", user.Name)
	}

	if !user.About.Valid || user.About.String != "Test about" {
		t.Errorf("expected about 'Test about', got %v", user.About)
	}

	if !user.Picture.Valid || user.Picture.String != "https://example.com/pic.jpg" {
		t.Errorf("expected picture 'https://example.com/pic.jpg', got %v", user.Picture)
	}

	if !user.Website.Valid || user.Website.String != "https://example.com" {
		t.Errorf("expected website 'https://example.com', got %v", user.Website)
	}

	if !user.Lud16.Valid || user.Lud16.String != "test@example.com" {
		t.Errorf("expected lud16 'test@example.com', got %v", user.Lud16)
	}

	// Test getting non-existent user
	nonExistentUser, err := store.GetUser(ctx, "0000000000000000000000000000000000000000000000000000000000000000")
	if err != nil {
		t.Fatalf("GetUser for non-existent user failed: %v", err)
	}

	if nonExistentUser != nil {
		t.Error("expected nil for non-existent user, got user")
	}
}

func TestSearchUsersByNamePrefix(t *testing.T) {
	ctx := context.Background()
	path := t.TempDir() + "/test.sqlite"
	db, err := testOpenDB(path)
	if err != nil {
		t.Fatal(err)
	}
	store, err := New(db)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	// Insert test users
	users := []struct {
		pubkey string
		name   string
		id     string
	}{
		{"0000000000000000000000000000000000000000000000000000000000000001", "Alice", "0000000000000000000000000000000000000000000000000000000000000001"},
		{"0000000000000000000000000000000000000000000000000000000000000002", "Bob", "0000000000000000000000000000000000000000000000000000000000000002"},
		{"0000000000000000000000000000000000000000000000000000000000000003", "Alice Smith", "0000000000000000000000000000000000000000000000000000000000000003"},
		{"0000000000000000000000000000000000000000000000000000000000000004", "Charlie", "0000000000000000000000000000000000000000000000000000000000000004"},
	}

	for _, user := range users {
		pubkey, _ := nostr.PubKeyFromHex(user.pubkey)
		id, _ := nostr.IDFromHex(user.id)
		event := &nostr.Event{
			ID:        id,
			PubKey:    pubkey,
			Kind:      0,
			CreatedAt: nostr.Timestamp(1000),
			Content:   `{"name":"` + user.name + `","about":"Test about"}`,
			Tags:      nil,
		}
		_, err = store.Save(ctx, event)
		if err != nil {
			t.Fatal(err)
		}
	}

	// Test searching for "Alice"
	results, err := store.SearchUsersByNamePrefix(ctx, "Alice", 10)
	if err != nil {
		t.Fatalf("SearchUsersByNamePrefix failed: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("expected 2 results for prefix 'Alice', got %d", len(results))
	}

	// Test searching for "Bob"
	results, err = store.SearchUsersByNamePrefix(ctx, "Bob", 10)
	if err != nil {
		t.Fatalf("SearchUsersByNamePrefix failed: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("expected 1 result for prefix 'Bob', got %d", len(results))
	}

	if results[0].Name.String != "Bob" {
		t.Errorf("expected name 'Bob', got %s", results[0].Name.String)
	}

	// Test searching for non-existent prefix
	results, err = store.SearchUsersByNamePrefix(ctx, "Xyz", 10)
	if err != nil {
		t.Fatalf("SearchUsersByNamePrefix failed: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("expected 0 results for prefix 'Xyz', got %d", len(results))
	}

	// Test limit functionality
	results, err = store.SearchUsersByNamePrefix(ctx, "Alice", 1)
	if err != nil {
		t.Fatalf("SearchUsersByNamePrefix failed: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("expected 1 result with limit 1, got %d", len(results))
	}
}

func TestUpsertRelayInfo(t *testing.T) {
	ctx := context.Background()
	path := t.TempDir() + "/test.sqlite"
	db, err := testOpenDB(path)
	if err != nil {
		t.Fatal(err)
	}
	store, err := New(db)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	relayURL := "wss://relay.example.com"
	fetchedAt := int64(1000)
	pubkey, _ := nostr.PubKeyFromHex("0000000000000000000000000000000000000000000000000000000000000001")
	self := pubkey
	info := nip11.RelayInformationDocument{
		Name:          "Test Relay",
		Description:   "A test relay",
		Banner:        "https://example.com/banner.jpg",
		Icon:          "https://example.com/icon.jpg",
		PubKey:        &pubkey,
		Self:          &self,
		Contact:       "admin@example.com",
		SupportedNIPs: []any{1, 9, 11, 16, 20, 33},
		Software:      "test-software",
		Version:       "1.0.0",
		PaymentsURL:   "https://example.com/payments",
	}

	// Test inserting relay info
	canonURL, err := store.UpsertRelayInfo(ctx, relayURL, fetchedAt, info)
	if err != nil {
		t.Fatalf("UpsertRelayInfo failed: %v", err)
	}

	expectedCanonURL := nostr.NormalizeURL(relayURL)
	if canonURL != expectedCanonURL {
		t.Errorf("expected canonical URL %s, got %s", expectedCanonURL, canonURL)
	}

	// Test updating relay info with newer timestamp
	newFetchedAt := int64(2000)
	info.Name = "Updated Test Relay"
	canonURL2, err := store.UpsertRelayInfo(ctx, relayURL, newFetchedAt, info)
	if err != nil {
		t.Fatalf("UpsertRelayInfo update failed: %v", err)
	}

	if canonURL2 != expectedCanonURL {
		t.Errorf("expected canonical URL %s, got %s", expectedCanonURL, canonURL2)
	}

	// Test updating with older timestamp (should not update)
	oldFetchedAt := int64(500)
	info.Name = "Old Test Relay"
	canonURL3, err := store.UpsertRelayInfo(ctx, relayURL, oldFetchedAt, info)
	if err != nil {
		t.Fatalf("UpsertRelayInfo with old timestamp failed: %v", err)
	}

	if canonURL3 != expectedCanonURL {
		t.Errorf("expected canonical URL %s, got %s", expectedCanonURL, canonURL3)
	}
}

func TestGetRelay(t *testing.T) {
	ctx := context.Background()
	path := t.TempDir() + "/test.sqlite"
	db, err := testOpenDB(path)
	if err != nil {
		t.Fatal(err)
	}
	store, err := New(db)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	relayURL := "wss://relay.example.com"
	fetchedAt := int64(1000)
	pubkey, _ := nostr.PubKeyFromHex("0000000000000000000000000000000000000000000000000000000000000001")
	self := pubkey
	info := nip11.RelayInformationDocument{
		Name:          "Test Relay",
		Description:   "A test relay",
		Banner:        "https://example.com/banner.jpg",
		Icon:          "https://example.com/icon.jpg",
		PubKey:        &pubkey,
		Self:          &self,
		Contact:       "admin@example.com",
		SupportedNIPs: []any{1, 9, 11, 16, 20, 33},
		Software:      "test-software",
		Version:       "1.0.0",
		PaymentsURL:   "https://example.com/payments",
	}

	// Insert relay info
	_, err = store.UpsertRelayInfo(ctx, relayURL, fetchedAt, info)
	if err != nil {
		t.Fatal(err)
	}

	// Test getting the relay
	relay, err := store.GetRelay(ctx, relayURL)
	if err != nil {
		t.Fatalf("GetRelay failed: %v", err)
	}

	if relay == nil {
		t.Fatal("expected relay, got nil")
	}

	expectedURL := nostr.NormalizeURL(relayURL)
	if relay.URL != expectedURL {
		t.Errorf("expected URL %s, got %s", expectedURL, relay.URL)
	}

	if relay.FetchedAt != fetchedAt {
		t.Errorf("expected fetched_at %d, got %d", fetchedAt, relay.FetchedAt)
	}

	if !relay.Name.Valid || relay.Name.String != "Test Relay" {
		t.Errorf("expected name 'Test Relay', got %v", relay.Name)
	}

	if !relay.Description.Valid || relay.Description.String != "A test relay" {
		t.Errorf("expected description 'A test relay', got %v", relay.Description)
	}

	if !relay.Banner.Valid || relay.Banner.String != "https://example.com/banner.jpg" {
		t.Errorf("expected banner 'https://example.com/banner.jpg', got %v", relay.Banner)
	}

	if !relay.Icon.Valid || relay.Icon.String != "https://example.com/icon.jpg" {
		t.Errorf("expected icon 'https://example.com/icon.jpg', got %v", relay.Icon)
	}

	if !relay.PubKey.Valid || relay.PubKey.String != pubkey.String() {
		t.Errorf("expected pubkey '%s', got %v", pubkey.String(), relay.PubKey)
	}

	if !relay.Self.Valid || relay.Self.String != pubkey.String() {
		t.Errorf("expected self '%s', got %v", pubkey.String(), relay.Self)
	}

	if !relay.Contact.Valid || relay.Contact.String != "admin@example.com" {
		t.Errorf("expected contact 'admin@example.com', got %v", relay.Contact)
	}

	if !relay.Software.Valid || relay.Software.String != "test-software" {
		t.Errorf("expected software 'test-software', got %v", relay.Software)
	}

	if !relay.Version.Valid || relay.Version.String != "1.0.0" {
		t.Errorf("expected version '1.0.0', got %v", relay.Version)
	}

	if !relay.PaymentsURL.Valid || relay.PaymentsURL.String != "https://example.com/payments" {
		t.Errorf("expected payments_url 'https://example.com/payments', got %v", relay.PaymentsURL)
	}

	// Test getting non-existent relay
	nonExistentRelay, err := store.GetRelay(ctx, "wss://nonexistent.example.com")
	if err != nil {
		t.Fatalf("GetRelay for non-existent relay failed: %v", err)
	}

	if nonExistentRelay != nil {
		t.Error("expected nil for non-existent relay, got relay")
	}
}

func TestSearchRelaysByNamePrefix(t *testing.T) {
	ctx := context.Background()
	path := t.TempDir() + "/test.sqlite"
	db, err := testOpenDB(path)
	if err != nil {
		t.Fatal(err)
	}
	store, err := New(db)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	// Insert test relays
	relays := []struct {
		url       string
		name      string
		fetchedAt int64
	}{
		{"wss://relay1.example.com", "Alpha Relay", 1000},
		{"wss://relay2.example.com", "Beta Relay", 1000},
		{"wss://relay3.example.com", "Alpha Test Relay", 1000},
		{"wss://relay4.example.com", "Gamma Relay", 1000},
	}

	for _, relay := range relays {
		info := nip11.RelayInformationDocument{
			Name: relay.name,
		}
		_, err = store.UpsertRelayInfo(ctx, relay.url, relay.fetchedAt, info)
		if err != nil {
			t.Fatal(err)
		}
	}

	// Test searching for "Alpha"
	results, err := store.SearchRelaysByNamePrefix(ctx, "Alpha", 10)
	if err != nil {
		t.Fatalf("SearchRelaysByNamePrefix failed: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("expected 2 results for prefix 'Alpha', got %d", len(results))
	}

	// Test searching for "Beta"
	results, err = store.SearchRelaysByNamePrefix(ctx, "Beta", 10)
	if err != nil {
		t.Fatalf("SearchRelaysByNamePrefix failed: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("expected 1 result for prefix 'Beta', got %d", len(results))
	}

	if results[0].Name.String != "Beta Relay" {
		t.Errorf("expected name 'Beta Relay', got %s", results[0].Name.String)
	}

	// Test searching for non-existent prefix
	results, err = store.SearchRelaysByNamePrefix(ctx, "Xyz", 10)
	if err != nil {
		t.Fatalf("SearchRelaysByNamePrefix failed: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("expected 0 results for prefix 'Xyz', got %d", len(results))
	}

	// Test limit functionality
	results, err = store.SearchRelaysByNamePrefix(ctx, "Alpha", 1)
	if err != nil {
		t.Fatalf("SearchRelaysByNamePrefix failed: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("expected 1 result with limit 1, got %d", len(results))
	}
}

func TestEscapeLike(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"normal", "normal"},
		{"%", "\\%"},
		{"_", "\\_"},
		{"\\", "\\\\"},
		{"%test%", "\\%test\\%"},
		{"_test_", "\\_test\\_"},
		{"\\%\\_", "\\\\\\%\\\\\\_"},
		{"", ""},
	}

	for _, test := range tests {
		result := escapeLike(test.input)
		if result != test.expected {
			t.Errorf("escapeLike(%q) = %q, expected %q", test.input, result, test.expected)
		}
	}
}
