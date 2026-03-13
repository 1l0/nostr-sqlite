CREATE TABLE IF NOT EXISTS events (
    id TEXT PRIMARY KEY,
    pubkey TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    kind INTEGER NOT NULL,
    tags JSONB NOT NULL,
    content TEXT NOT NULL,
    sig TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS time_idx ON events(created_at DESC, id ASC);
CREATE INDEX IF NOT EXISTS kind_sorted_idx ON events(kind, created_at DESC, id ASC);
CREATE INDEX IF NOT EXISTS pubkey_kind_sorted_idx ON events(pubkey, kind, created_at DESC, id ASC);

CREATE TABLE IF NOT EXISTS users (
    pubkey TEXT PRIMARY KEY,
    name TEXT,
    about TEXT,
    picture TEXT,
    website TEXT,
    banner TEXT,
    bot INTEGER,
    lud16 TEXT,

    event_id TEXT NOT NULL,
    created_at INTEGER NOT NULL,

    FOREIGN KEY (event_id) REFERENCES events(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS users_name_idx ON users(name);

CREATE TRIGGER IF NOT EXISTS users_ai AFTER INSERT ON events
WHEN NEW.kind = 0 AND json_valid(NEW.content)
BEGIN
INSERT INTO users (pubkey, name, about, picture, website, banner, bot, lud16, event_id, created_at)
VALUES (
    NEW.pubkey,
    json_extract(NEW.content, '$.name'),
    json_extract(NEW.content, '$.about'),
    json_extract(NEW.content, '$.picture'),
    json_extract(NEW.content, '$.website'),
    json_extract(NEW.content, '$.banner'),
    json_extract(NEW.content, '$.bot'),
    json_extract(NEW.content, '$.lud16'),
    NEW.id,
    NEW.created_at
)
ON CONFLICT(pubkey) DO UPDATE SET
    name = excluded.name,
    about = excluded.about,
    picture = excluded.picture,
    website = excluded.website,
    banner = excluded.banner,
    bot = excluded.bot,
    lud16 = excluded.lud16,
    event_id = excluded.event_id,
    created_at = excluded.created_at
WHERE excluded.created_at > users.created_at;
END;

CREATE TABLE IF NOT EXISTS relays (
    url TEXT PRIMARY KEY,
    fetched_at INTEGER NOT NULL,

    name TEXT,
    description TEXT,
    banner TEXT,
    icon TEXT,
    pubkey TEXT,
    self TEXT,
    contact TEXT,
    supported_nips JSON,
    software TEXT,
    version TEXT,

    limitation JSON,
    payments_url TEXT,
    fees JSON
);

CREATE INDEX IF NOT EXISTS relays_name_idx ON relays(name);

CREATE TABLE IF NOT EXISTS tags (
    event_id TEXT NOT NULL,
    key TEXT NOT NULL,
    value TEXT NOT NULL,
    
    PRIMARY KEY (key, value, event_id),
    FOREIGN KEY (event_id) REFERENCES events(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS tags_event_id_idx ON tags(event_id);

CREATE TRIGGER IF NOT EXISTS d_tags_ai AFTER INSERT ON events
WHEN NEW.kind BETWEEN 30000 AND 39999 
BEGIN
INSERT INTO tags (event_id, key, value)
    SELECT NEW.id, 'd', json_extract(value, '$[1]')
    FROM json_each(NEW.tags)
    WHERE json_type(value) = 'array' AND json_array_length(value) > 1 AND json_extract(value, '$[0]') = 'd'
    LIMIT 1;
END;