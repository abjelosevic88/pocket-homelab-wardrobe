CREATE TABLE users (
  id            INTEGER PRIMARY KEY AUTOINCREMENT,
  email         TEXT NOT NULL UNIQUE,
  name          TEXT NOT NULL DEFAULT '',
  password_hash TEXT NOT NULL DEFAULT '',
  role          TEXT NOT NULL DEFAULT 'member',
  created_at    TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  updated_at    TEXT
);

CREATE TABLE sessions (
  id         TEXT PRIMARY KEY,
  user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  expires_at TEXT NOT NULL,
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);

CREATE TABLE api_tokens (
  id           INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id      INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name         TEXT NOT NULL DEFAULT '',
  token_hash   TEXT NOT NULL UNIQUE,
  prefix       TEXT NOT NULL DEFAULT '',
  last_used_at TEXT,
  created_at   TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);

-- Box-wide settings (title, setup flag); everything else is personal.
CREATE TABLE settings (
  key   TEXT PRIMARY KEY,
  value TEXT NOT NULL DEFAULT ''
);

CREATE TABLE user_settings (
  user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  key     TEXT NOT NULL,
  value   TEXT NOT NULL DEFAULT '',
  PRIMARY KEY (user_id, key)
);

CREATE TABLE cache (
  key        TEXT PRIMARY KEY,
  value      TEXT NOT NULL DEFAULT '',
  fetched_at TEXT NOT NULL
);




-- ---------- Pocket Wardrobe: everything below is per person ----------
-- Money in minor units (cents) with its currency; dates YYYY-MM-DD.

-- Pictures: of a piece, of a whole look (a mirror photo), of a shop find.
-- The browser shrinks them first (1600 px and a 480 px thumbnail, JPEG).
CREATE TABLE photos (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  kind       TEXT NOT NULL DEFAULT 'item',   -- item | look | check
  mime       TEXT NOT NULL DEFAULT 'image/jpeg',
  data       BLOB NOT NULL,
  thumb      BLOB,
  width      INTEGER NOT NULL DEFAULT 0,
  height     INTEGER NOT NULL DEFAULT 0,
  bytes      INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
CREATE INDEX photos_user ON photos(user_id, kind);

-- A piece of clothing (or shoes, a bag, a belt…).
-- colors: [{"hex":"#1f2a44","name":"navy"}, …], the main one first.
CREATE TABLE items (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id     INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name        TEXT NOT NULL DEFAULT '',
  category    TEXT NOT NULL DEFAULT 'top',    -- top | knit | bottom | dress | outerwear | shoes | bag | accessory | active | other
  kind        TEXT NOT NULL DEFAULT '',       -- t-shirt, shirt, jeans, sneakers… (see style.Kinds)
  colors      TEXT NOT NULL DEFAULT '[]',
  pattern     TEXT NOT NULL DEFAULT 'plain',  -- plain | stripes | checks | dots | floral | print | other
  material    TEXT NOT NULL DEFAULT '',
  fit         TEXT NOT NULL DEFAULT '',       -- slim | regular | relaxed | oversized
  seasons     TEXT NOT NULL DEFAULT '',       -- "spring,summer,autumn,winter"
  warmth      INTEGER NOT NULL DEFAULT 2,     -- 1 very light … 5 very warm
  formality   INTEGER NOT NULL DEFAULT 2,     -- 1 sporty … 5 black tie
  styles      TEXT NOT NULL DEFAULT '',       -- style ids, comma separated
  rain        INTEGER NOT NULL DEFAULT 0,     -- keeps rain out
  brand       TEXT NOT NULL DEFAULT '',
  size        TEXT NOT NULL DEFAULT '',
  price       INTEGER,
  currency    TEXT NOT NULL DEFAULT '',
  bought_on   TEXT NOT NULL DEFAULT '',
  shop        TEXT NOT NULL DEFAULT '',
  care        TEXT NOT NULL DEFAULT '',
  notes       TEXT NOT NULL DEFAULT '',
  status      TEXT NOT NULL DEFAULT 'ready',  -- ready | laundry | repair | lent | stored
  status_note TEXT NOT NULL DEFAULT '',       -- who has it, what needs fixing
  status_at   TEXT NOT NULL DEFAULT '',
  favourite   INTEGER NOT NULL DEFAULT 0,
  gone        TEXT NOT NULL DEFAULT '',       -- '' | donated | sold | thrown | lost
  gone_on     TEXT NOT NULL DEFAULT '',
  sold_for    INTEGER,
  photo_id    INTEGER REFERENCES photos(id) ON DELETE SET NULL,
  source      TEXT NOT NULL DEFAULT '',       -- '' | ai | money | parcels | wishlist | <app slug>
  source_ref  TEXT NOT NULL DEFAULT '',
  created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  updated_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
CREATE INDEX items_user ON items(user_id, gone, category);
CREATE INDEX items_source ON items(user_id, source, source_ref);

-- A saved outfit: pieces that go together.
CREATE TABLE outfits (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name       TEXT NOT NULL DEFAULT '',
  item_ids   TEXT NOT NULL DEFAULT '[]',
  style      TEXT NOT NULL DEFAULT '',
  occasion   TEXT NOT NULL DEFAULT '',
  seasons    TEXT NOT NULL DEFAULT '',
  rating     INTEGER NOT NULL DEFAULT 0,      -- 0 none, 1–5
  favourite  INTEGER NOT NULL DEFAULT 0,
  notes      TEXT NOT NULL DEFAULT '',
  photo_id   INTEGER REFERENCES photos(id) ON DELETE SET NULL,  -- you wearing it
  made_by    TEXT NOT NULL DEFAULT 'me',      -- me | ideas | ai | <app slug>
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
CREATE INDEX outfits_user ON outfits(user_id);

-- What was worn on a day (and, for a day ahead, what is planned).
-- weather: {"t_max":…, "t_min":…, "condition":…} as it was.
CREATE TABLE wears (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  date       TEXT NOT NULL,
  planned    INTEGER NOT NULL DEFAULT 0,      -- 1 = a plan for that day, not worn yet
  outfit_id  INTEGER REFERENCES outfits(id) ON DELETE SET NULL,
  item_ids   TEXT NOT NULL DEFAULT '[]',
  occasion   TEXT NOT NULL DEFAULT '',
  event      TEXT NOT NULL DEFAULT '',        -- the calendar event it was for
  event_ref  TEXT NOT NULL DEFAULT '',
  weather    TEXT NOT NULL DEFAULT '',
  feel       INTEGER NOT NULL DEFAULT 0,      -- how it felt, 1–5
  notes      TEXT NOT NULL DEFAULT '',
  photo_id   INTEGER REFERENCES photos(id) ON DELETE SET NULL,
  source     TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
CREATE INDEX wears_user ON wears(user_id, date);

-- One row per piece per wear, for the counts.
CREATE TABLE wear_items (
  wear_id INTEGER NOT NULL REFERENCES wears(id) ON DELETE CASCADE,
  item_id INTEGER NOT NULL REFERENCES items(id) ON DELETE CASCADE,
  user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  date    TEXT NOT NULL,
  planned INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (wear_id, item_id)
);
CREATE INDEX wear_items_item ON wear_items(user_id, item_id, date);

-- Things worth buying: from the gaps in the wardrobe, from the AI, or kept
-- from "before you buy".
CREATE TABLE ideas (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  title      TEXT NOT NULL DEFAULT '',
  category   TEXT NOT NULL DEFAULT '',
  kind       TEXT NOT NULL DEFAULT '',
  color_hex  TEXT NOT NULL DEFAULT '',
  color_name TEXT NOT NULL DEFAULT '',
  why        TEXT NOT NULL DEFAULT '',
  style      TEXT NOT NULL DEFAULT '',
  unlocks    INTEGER NOT NULL DEFAULT 0,      -- new outfits it makes with what you have
  price_hint TEXT NOT NULL DEFAULT '',
  url        TEXT NOT NULL DEFAULT '',
  photo_id   INTEGER REFERENCES photos(id) ON DELETE SET NULL,
  status     TEXT NOT NULL DEFAULT 'idea',    -- idea | wish | bought | dismissed
  made_by    TEXT NOT NULL DEFAULT 'gaps',    -- gaps | ai | check
  wish_ref   TEXT NOT NULL DEFAULT '',        -- the wish in Pocket Wishlist
  wish_url   TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
CREATE INDEX ideas_user ON ideas(user_id, status);

-- Packing for a trip (from Pocket Travel or typed in).
-- items: [{"item_id":12,"qty":1,"packed":false}, …]
CREATE TABLE packs (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id     INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name        TEXT NOT NULL DEFAULT '',
  destination TEXT NOT NULL DEFAULT '',
  lat         REAL,
  lng         REAL,
  start_date  TEXT NOT NULL DEFAULT '',
  end_date    TEXT NOT NULL DEFAULT '',
  purpose     TEXT NOT NULL DEFAULT 'holiday', -- holiday | city | business | beach | ski | outdoor | wedding | other
  items       TEXT NOT NULL DEFAULT '[]',
  extras      TEXT NOT NULL DEFAULT '[]',      -- [{"text":"Socks ×5","packed":false}]
  weather     TEXT NOT NULL DEFAULT '',
  notes       TEXT NOT NULL DEFAULT '',
  trip_ref    TEXT NOT NULL DEFAULT '',
  trip_url    TEXT NOT NULL DEFAULT '',
  created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
CREATE INDEX packs_user ON packs(user_id, start_date);
