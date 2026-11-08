package store

import (
	"context"
	"database/sql"
	"encoding/json"
)

// CreatePhoto stores a picture.
func (s *Store) CreatePhoto(ctx context.Context, p *Photo) error {
	p.Bytes = len(p.Data)
	res, err := s.DB.ExecContext(ctx, `INSERT INTO photos (user_id, kind, mime, data, thumb, width, height, bytes) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		p.UserID, p.Kind, p.Mime, p.Data, p.Thumb, p.Width, p.Height, p.Bytes)
	if err != nil {
		return err
	}
	p.ID, _ = res.LastInsertId()
	return nil
}

// GetPhoto returns one of a person's pictures (thumb = the small one, when there is one).
func (s *Store) GetPhoto(ctx context.Context, uid, id int64, thumb bool) (*Photo, error) {
	var p Photo
	var t []byte
	err := s.DB.QueryRowContext(ctx, `SELECT id, user_id, kind, mime, data, thumb, width, height, bytes, created_at FROM photos WHERE user_id = ? AND id = ?`, uid, id).
		Scan(&p.ID, &p.UserID, &p.Kind, &p.Mime, &p.Data, &t, &p.Width, &p.Height, &p.Bytes, &p.CreatedAt)
	if err != nil {
		return nil, notFound(err)
	}
	if thumb && len(t) > 0 {
		p.Data = t
		p.Mime = "image/jpeg"
	}
	return &p, nil
}

// DeletePhotoIfUnused removes a picture nothing points at any more.
func (s *Store) DeletePhotoIfUnused(ctx context.Context, uid, id int64) error {
	var n int
	_ = s.DB.QueryRowContext(ctx, `SELECT (SELECT COUNT(1) FROM items WHERE photo_id = ?) + (SELECT COUNT(1) FROM outfits WHERE photo_id = ?) + (SELECT COUNT(1) FROM wears WHERE photo_id = ?) + (SELECT COUNT(1) FROM ideas WHERE photo_id = ?)`, id, id, id, id).Scan(&n)
	if n > 0 {
		return nil
	}
	_, err := s.DB.ExecContext(ctx, `DELETE FROM photos WHERE user_id = ? AND id = ?`, uid, id)
	return err
}

// PurgePhotos removes pictures uploaded more than a day ago that nothing uses.
func (s *Store) PurgePhotos(ctx context.Context) (int64, error) {
	res, err := s.DB.ExecContext(ctx, `DELETE FROM photos WHERE created_at < strftime('%Y-%m-%dT%H:%M:%fZ','now','-1 day')
		AND id NOT IN (SELECT photo_id FROM items WHERE photo_id IS NOT NULL) AND id NOT IN (SELECT photo_id FROM outfits WHERE photo_id IS NOT NULL)
		AND id NOT IN (SELECT photo_id FROM wears WHERE photo_id IS NOT NULL) AND id NOT IN (SELECT photo_id FROM ideas WHERE photo_id IS NOT NULL)`)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// PhotoBytes is the room a person's pictures take.
func (s *Store) PhotoBytes(ctx context.Context, uid int64) int64 {
	var n sql.NullInt64
	_ = s.DB.QueryRowContext(ctx, `SELECT SUM(bytes + COALESCE(length(thumb), 0)) FROM photos WHERE user_id = ?`, uid).Scan(&n)
	return n.Int64
}

// ---- ideas ----

const ideaCols = `id, user_id, title, category, kind, color_hex, color_name, why, style, unlocks, price_hint, url, photo_id, status, made_by, wish_ref, wish_url, created_at`

func scanIdea(sc interface{ Scan(...any) error }) (*Idea, error) {
	var i Idea
	var photo sql.NullInt64
	if err := sc.Scan(&i.ID, &i.UserID, &i.Title, &i.Category, &i.Kind, &i.ColorHex, &i.ColorName, &i.Why, &i.Style, &i.Unlocks, &i.PriceHint, &i.URL, &photo, &i.Status, &i.MadeBy, &i.WishRef, &i.WishURL, &i.CreatedAt); err != nil {
		return nil, err
	}
	if photo.Valid {
		i.PhotoID = &photo.Int64
	}
	return &i, nil
}

// ListIdeas lists a person's kept ideas (all states, or one).
func (s *Store) ListIdeas(ctx context.Context, uid int64, status string) ([]*Idea, error) {
	q := `SELECT ` + ideaCols + ` FROM ideas WHERE user_id = ?`
	args := []any{uid}
	if status != "" {
		q += ` AND status = ?`
		args = append(args, status)
	}
	rows, err := s.DB.QueryContext(ctx, q+` ORDER BY created_at DESC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*Idea{}
	for rows.Next() {
		i, err := scanIdea(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, i)
	}
	return out, rows.Err()
}

// GetIdea returns one of a person's ideas.
func (s *Store) GetIdea(ctx context.Context, uid, id int64) (*Idea, error) {
	i, err := scanIdea(s.DB.QueryRowContext(ctx, `SELECT `+ideaCols+` FROM ideas WHERE user_id = ? AND id = ?`, uid, id))
	return i, notFound(err)
}

// CreateIdea keeps an idea.
func (s *Store) CreateIdea(ctx context.Context, i *Idea) error {
	if i.Status == "" {
		i.Status = "idea"
	}
	res, err := s.DB.ExecContext(ctx, `INSERT INTO ideas (user_id, title, category, kind, color_hex, color_name, why, style, unlocks, price_hint, url, photo_id, status, made_by, wish_ref, wish_url) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		i.UserID, i.Title, i.Category, i.Kind, i.ColorHex, i.ColorName, i.Why, i.Style, i.Unlocks, i.PriceHint, i.URL, nullI(i.PhotoID), i.Status, i.MadeBy, i.WishRef, i.WishURL)
	if err != nil {
		return err
	}
	i.ID, _ = res.LastInsertId()
	return nil
}

// UpdateIdea saves an idea.
func (s *Store) UpdateIdea(ctx context.Context, i *Idea) error {
	res, err := s.DB.ExecContext(ctx, `UPDATE ideas SET title = ?, category = ?, kind = ?, color_hex = ?, color_name = ?, why = ?, style = ?, unlocks = ?, price_hint = ?, url = ?, photo_id = ?, status = ?, wish_ref = ?, wish_url = ?
		WHERE user_id = ? AND id = ?`, i.Title, i.Category, i.Kind, i.ColorHex, i.ColorName, i.Why, i.Style, i.Unlocks, i.PriceHint, i.URL, nullI(i.PhotoID), i.Status, i.WishRef, i.WishURL, i.UserID, i.ID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteIdea removes an idea.
func (s *Store) DeleteIdea(ctx context.Context, uid, id int64) error {
	i, err := s.GetIdea(ctx, uid, id)
	if err != nil {
		return err
	}
	if _, err := s.DB.ExecContext(ctx, `DELETE FROM ideas WHERE user_id = ? AND id = ?`, uid, id); err != nil {
		return err
	}
	if i.PhotoID != nil {
		_ = s.DeletePhotoIfUnused(ctx, uid, *i.PhotoID)
	}
	return nil
}

// ---- packing ----

const packCols = `id, user_id, name, destination, lat, lng, start_date, end_date, purpose, items, extras, weather, notes, trip_ref, trip_url, created_at`

func scanPack(sc interface{ Scan(...any) error }) (*Pack, error) {
	var p Pack
	var lat, lng sql.NullFloat64
	var items, extras, weather string
	if err := sc.Scan(&p.ID, &p.UserID, &p.Name, &p.Destination, &lat, &lng, &p.StartDate, &p.EndDate, &p.Purpose, &items, &extras, &weather, &p.Notes, &p.TripRef, &p.TripURL, &p.CreatedAt); err != nil {
		return nil, err
	}
	if lat.Valid && lng.Valid {
		p.Lat, p.Lng = &lat.Float64, &lng.Float64
	}
	_ = json.Unmarshal([]byte(items), &p.Items)
	_ = json.Unmarshal([]byte(extras), &p.Extras)
	if p.Items == nil {
		p.Items = []PackItem{}
	}
	if p.Extras == nil {
		p.Extras = []PackExtra{}
	}
	if weather != "" && json.Valid([]byte(weather)) {
		p.Weather = json.RawMessage(weather)
	}
	return &p, nil
}

// ListPacks lists a person's packing lists, the next trip first.
func (s *Store) ListPacks(ctx context.Context, uid int64) ([]*Pack, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT `+packCols+` FROM packs WHERE user_id = ? ORDER BY CASE WHEN end_date >= date('now') OR end_date = '' THEN 0 ELSE 1 END, start_date`, uid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*Pack{}
	for rows.Next() {
		p, err := scanPack(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// GetPack returns one of a person's packing lists.
func (s *Store) GetPack(ctx context.Context, uid, id int64) (*Pack, error) {
	p, err := scanPack(s.DB.QueryRowContext(ctx, `SELECT `+packCols+` FROM packs WHERE user_id = ? AND id = ?`, uid, id))
	return p, notFound(err)
}

func nullF(p *float64) any {
	if p == nil {
		return nil
	}
	return *p
}

// CreatePack adds a packing list.
func (s *Store) CreatePack(ctx context.Context, p *Pack) error {
	items, _ := json.Marshal(p.Items)
	extras, _ := json.Marshal(p.Extras)
	if p.Purpose == "" {
		p.Purpose = "holiday"
	}
	res, err := s.DB.ExecContext(ctx, `INSERT INTO packs (user_id, name, destination, lat, lng, start_date, end_date, purpose, items, extras, weather, notes, trip_ref, trip_url) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.UserID, p.Name, p.Destination, nullF(p.Lat), nullF(p.Lng), p.StartDate, p.EndDate, p.Purpose, string(items), string(extras), rawOr(p.Weather), p.Notes, p.TripRef, p.TripURL)
	if err != nil {
		return err
	}
	p.ID, _ = res.LastInsertId()
	return nil
}

// UpdatePack saves a packing list.
func (s *Store) UpdatePack(ctx context.Context, p *Pack) error {
	items, _ := json.Marshal(p.Items)
	extras, _ := json.Marshal(p.Extras)
	res, err := s.DB.ExecContext(ctx, `UPDATE packs SET name = ?, destination = ?, lat = ?, lng = ?, start_date = ?, end_date = ?, purpose = ?, items = ?, extras = ?, weather = ?, notes = ?, trip_ref = ?, trip_url = ?
		WHERE user_id = ? AND id = ?`, p.Name, p.Destination, nullF(p.Lat), nullF(p.Lng), p.StartDate, p.EndDate, p.Purpose, string(items), string(extras), rawOr(p.Weather), p.Notes, p.TripRef, p.TripURL, p.UserID, p.ID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeletePack removes a packing list.
func (s *Store) DeletePack(ctx context.Context, uid, id int64) error {
	res, err := s.DB.ExecContext(ctx, `DELETE FROM packs WHERE user_id = ? AND id = ?`, uid, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}
