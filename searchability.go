package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

type User struct {
	PubKey    string
	Name      sql.NullString
	About     sql.NullString
	Picture   sql.NullString
	Website   sql.NullString
	Banner    sql.NullString
	Bot       sql.NullInt64
	Lud16     sql.NullString
	EventID   string
	CreatedAt int64
}

// RelayInfo is a superset-friendly representation of NIP-11.
// Unknown fields should be stored in Raw.
type RelayInfo struct {
	Name           *string `json:"name,omitempty"`
	Description    *string `json:"description,omitempty"`
	Banner         *string `json:"banner,omitempty"`
	Icon           *string `json:"icon,omitempty"`
	PubKey         *string `json:"pubkey,omitempty"`
	Self           *string `json:"self,omitempty"`
	Contact        *string `json:"contact,omitempty"`
	SupportedNIPs  []int   `json:"supported_nips,omitempty"`
	Software       *string `json:"software,omitempty"`
	Version        *string `json:"version,omitempty"`
	TermsOfService *string `json:"terms_of_service,omitempty"`

	Limitation  any     `json:"limitation,omitempty"`
	PaymentsURL *string `json:"payments_url,omitempty"`
	Fees        any     `json:"fees,omitempty"`

	Raw json.RawMessage `json:"-"`
}

type Relay struct {
	URL       string
	FetchedAt int64

	Name           sql.NullString
	Description    sql.NullString
	Banner         sql.NullString
	Icon           sql.NullString
	PubKey         sql.NullString
	Self           sql.NullString
	Contact        sql.NullString
	SupportedNIPs  []byte
	Software       sql.NullString
	Version        sql.NullString
	TermsOfService sql.NullString

	Limitation  []byte
	PaymentsURL sql.NullString
	Fees        []byte
	Raw         []byte
}

func CanonicalRelayURL(input string) (string, error) {
	s := strings.TrimSpace(input)
	if s == "" {
		return "", fmt.Errorf("empty relay url")
	}

	// tolerate inputs without scheme
	if !strings.Contains(s, "://") {
		s = "https://" + s
	}

	u, err := url.Parse(s)
	if err != nil {
		return "", err
	}

	switch strings.ToLower(u.Scheme) {
	case "ws":
		u.Scheme = "http"
	case "wss":
		u.Scheme = "https"
	case "http", "https":
		// keep
	default:
		// if unknown, keep but still normalize host casing etc.
	}

	u.Fragment = ""
	u.RawQuery = ""
	u.User = nil

	u.Host = strings.ToLower(u.Host)
	if strings.HasSuffix(u.Host, ":80") && u.Scheme == "http" {
		u.Host = strings.TrimSuffix(u.Host, ":80")
	}
	if strings.HasSuffix(u.Host, ":443") && u.Scheme == "https" {
		u.Host = strings.TrimSuffix(u.Host, ":443")
	}

	u.Path = strings.TrimRight(u.Path, "/")

	return u.String(), nil
}

func (s *Store) GetUser(ctx context.Context, pubkey string) (*User, error) {
	row := s.DB.QueryRowContext(ctx, `
SELECT pubkey, name, about, picture, website, banner, bot, lud16, event_id, created_at
FROM users
WHERE pubkey = $1`, pubkey)

	u := &User{}
	if err := row.Scan(&u.PubKey, &u.Name, &u.About, &u.Picture, &u.Website, &u.Banner, &u.Bot, &u.Lud16, &u.EventID, &u.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return u, nil
}

// SearchUsersByNamePrefix finds users whose name starts with prefix.
func (s *Store) SearchUsersByNamePrefix(ctx context.Context, prefix string, limit int) ([]User, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.DB.QueryContext(ctx, `
SELECT pubkey, name, about, picture, website, banner, bot, lud16, event_id, created_at
FROM users
WHERE name LIKE $1 ESCAPE '\'
ORDER BY name ASC
LIMIT $2`, escapeLike(prefix)+"%", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]User, 0, 8)
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.PubKey, &u.Name, &u.About, &u.Picture, &u.Website, &u.Banner, &u.Bot, &u.Lud16, &u.EventID, &u.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func (s *Store) UpsertRelayInfo(ctx context.Context, relayURL string, fetchedAt int64, info RelayInfo) (string, error) {
	canon, err := CanonicalRelayURL(relayURL)
	if err != nil {
		return "", err
	}

	raw := info.Raw
	if len(raw) == 0 {
		// best-effort: marshal known fields, allowing extra fields to be lost if not provided in Raw.
		raw, err = json.Marshal(info)
		if err != nil {
			return "", err
		}
	}

	var supportedNIPs any
	if info.SupportedNIPs != nil {
		supportedNIPs, err = json.Marshal(info.SupportedNIPs)
		if err != nil {
			return "", err
		}
	}

	var limitation any
	if info.Limitation != nil {
		limitation, err = json.Marshal(info.Limitation)
		if err != nil {
			return "", err
		}
	}

	var fees any
	if info.Fees != nil {
		fees, err = json.Marshal(info.Fees)
		if err != nil {
			return "", err
		}
	}

	_, err = s.DB.ExecContext(ctx, `
INSERT INTO relays (
  url, fetched_at,
  name, description, banner, icon, pubkey, self, contact, supported_nips, software, version, terms_of_service,
  limitation, payments_url, fees, raw
) VALUES (
  $1, $2,
  $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13,
  $14, $15, $16, $17
)
ON CONFLICT(url) DO UPDATE SET
  fetched_at = excluded.fetched_at,
  name = excluded.name,
  description = excluded.description,
  banner = excluded.banner,
  icon = excluded.icon,
  pubkey = excluded.pubkey,
  self = excluded.self,
  contact = excluded.contact,
  supported_nips = excluded.supported_nips,
  software = excluded.software,
  version = excluded.version,
  terms_of_service = excluded.terms_of_service,
  limitation = excluded.limitation,
  payments_url = excluded.payments_url,
  fees = excluded.fees,
  raw = excluded.raw
WHERE excluded.fetched_at > relays.fetched_at
`, canon, fetchedAt,
		info.Name, info.Description, info.Banner, info.Icon, info.PubKey, info.Self, info.Contact, supportedNIPs, info.Software, info.Version, info.TermsOfService,
		limitation, info.PaymentsURL, fees, raw)
	if err != nil {
		return "", err
	}

	s.checkOptimize(ctx)
	return canon, nil
}

func (s *Store) GetRelay(ctx context.Context, relayURL string) (*Relay, error) {
	canon, err := CanonicalRelayURL(relayURL)
	if err != nil {
		return nil, err
	}

	row := s.DB.QueryRowContext(ctx, `
SELECT url, fetched_at, name, description, banner, icon, pubkey, self, contact, supported_nips, software, version, terms_of_service,
       limitation, payments_url, fees, raw
FROM relays
WHERE url = $1`, canon)

	var r Relay
	if err := row.Scan(
		&r.URL, &r.FetchedAt, &r.Name, &r.Description, &r.Banner, &r.Icon, &r.PubKey, &r.Self, &r.Contact, &r.SupportedNIPs, &r.Software, &r.Version, &r.TermsOfService,
		&r.Limitation, &r.PaymentsURL, &r.Fees, &r.Raw,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &r, nil
}

func (s *Store) SearchRelaysByNamePrefix(ctx context.Context, prefix string, limit int) ([]Relay, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.DB.QueryContext(ctx, `
SELECT url, fetched_at, name, description, banner, icon, pubkey, self, contact, supported_nips, software, version, terms_of_service,
       limitation, payments_url, fees, raw
FROM relays
WHERE name LIKE $1 ESCAPE '\'
ORDER BY name ASC
LIMIT $2`, escapeLike(prefix)+"%", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Relay, 0, 8)
	for rows.Next() {
		var r Relay
		if err := rows.Scan(
			&r.URL, &r.FetchedAt, &r.Name, &r.Description, &r.Banner, &r.Icon, &r.PubKey, &r.Self, &r.Contact, &r.SupportedNIPs, &r.Software, &r.Version, &r.TermsOfService,
			&r.Limitation, &r.PaymentsURL, &r.Fees, &r.Raw,
		); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func escapeLike(s string) string {
	// escape \ first to avoid double-escaping
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return s
}

