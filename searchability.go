package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"

	"fiatjaf.com/nostr"
	"fiatjaf.com/nostr/nip11"
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

func (s *Store) UpsertRelayInfo(ctx context.Context, relayURL string, fetchedAt int64, info nip11.RelayInformationDocument) (string, error) {
	var err error
	canon := nostr.NormalizeURL(relayURL)

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

	var pubkeyStr, selfStr string
	if info.PubKey != nil {
		pubkeyStr = info.PubKey.String()
	}
	if info.Self != nil {
		selfStr = info.Self.String()
	}

	_, err = s.DB.ExecContext(ctx, `
INSERT INTO relays (
  url, fetched_at,
  name, description, banner, icon, pubkey, self, contact, supported_nips, software, version,
  limitation, payments_url, fees
) VALUES (
  $1, $2,
  $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, 
  $13, $14, $15
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
  limitation = excluded.limitation,
  payments_url = excluded.payments_url,
  fees = excluded.fees
WHERE excluded.fetched_at > relays.fetched_at
`, canon, fetchedAt,
		info.Name, info.Description, info.Banner, info.Icon, pubkeyStr, selfStr, info.Contact, supportedNIPs, info.Software, info.Version,
		limitation, info.PaymentsURL, fees)
	if err != nil {
		return "", err
	}

	s.checkOptimize(ctx)
	return canon, nil
}

func (s *Store) GetRelay(ctx context.Context, relayURL string) (*Relay, error) {
	canon := nostr.NormalizeURL(relayURL)

	row := s.DB.QueryRowContext(ctx, `
SELECT url, fetched_at, name, description, banner, icon, pubkey, self, contact, supported_nips, software, version,
       limitation, payments_url, fees
FROM relays
WHERE url = $1`, canon)

	var r Relay
	if err := row.Scan(
		&r.URL, &r.FetchedAt, &r.Name, &r.Description, &r.Banner, &r.Icon, &r.PubKey, &r.Self, &r.Contact, &r.SupportedNIPs, &r.Software, &r.Version,
		&r.Limitation, &r.PaymentsURL, &r.Fees,
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
SELECT url, fetched_at, name, description, banner, icon, pubkey, self, contact, supported_nips, software, version,
       limitation, payments_url, fees
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
			&r.URL, &r.FetchedAt, &r.Name, &r.Description, &r.Banner, &r.Icon, &r.PubKey, &r.Self, &r.Contact, &r.SupportedNIPs, &r.Software, &r.Version,
			&r.Limitation, &r.PaymentsURL, &r.Fees,
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
