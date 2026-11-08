package store

import (
	"context"
	"database/sql"
	"encoding/json"
)

const outfitCols = `o.id, o.user_id, o.name, o.item_ids, o.style, o.occasion, o.seasons, o.rating, o.favourite, o.notes, o.photo_id, o.made_by, o.created_at, o.updated_at,
	(SELECT COUNT(1) FROM wears w WHERE w.outfit_id = o.id AND w.planned = 0), COALESCE((SELECT MAX(w.date) FROM wears w WHERE w.outfit_id = o.id AND w.planned = 0), '')`

func scanOutfit(sc interface{ Scan(...any) error }) (*Outfit, error) {
	var o Outfit
	var items, seasons string
	var fav int
	var photo sql.NullInt64
	if err := sc.Scan(&o.ID, &o.UserID, &o.Name, &items, &o.Style, &o.Occasion, &seasons, &o.Rating, &fav, &o.Notes, &photo, &o.MadeBy, &o.CreatedAt, &o.UpdatedAt, &o.Wears, &o.LastWorn); err != nil {
		return nil, err
	}
	o.ItemIDs, o.Seasons, o.Favourite = ids(items), split(seasons), fav == 1
	if photo.Valid {
		o.PhotoID = &photo.Int64
	}
	return &o, nil
}

// ListOutfits lists a person's saved outfits, newest first.
func (s *Store) ListOutfits(ctx context.Context, uid int64) ([]*Outfit, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT `+outfitCols+` FROM outfits o WHERE o.user_id = ? ORDER BY o.favourite DESC, o.updated_at DESC`, uid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*Outfit{}
	for rows.Next() {
		o, err := scanOutfit(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// GetOutfit returns one of a person's outfits.
func (s *Store) GetOutfit(ctx context.Context, uid, id int64) (*Outfit, error) {
	o, err := scanOutfit(s.DB.QueryRowContext(ctx, `SELECT `+outfitCols+` FROM outfits o WHERE o.user_id = ? AND o.id = ?`, uid, id))
	return o, notFound(err)
}

// CreateOutfit saves an outfit.
func (s *Store) CreateOutfit(ctx context.Context, o *Outfit) error {
	items, _ := json.Marshal(o.ItemIDs)
	if o.MadeBy == "" {
		o.MadeBy = "me"
	}
	res, err := s.DB.ExecContext(ctx, `INSERT INTO outfits (user_id, name, item_ids, style, occasion, seasons, rating, favourite, notes, photo_id, made_by) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		o.UserID, o.Name, string(items), o.Style, o.Occasion, csv(o.Seasons), o.Rating, boolInt(o.Favourite), o.Notes, nullI(o.PhotoID), o.MadeBy)
	if err != nil {
		return err
	}
	o.ID, _ = res.LastInsertId()
	return nil
}

// UpdateOutfit saves changes to an outfit.
func (s *Store) UpdateOutfit(ctx context.Context, o *Outfit) error {
	items, _ := json.Marshal(o.ItemIDs)
	res, err := s.DB.ExecContext(ctx, `UPDATE outfits SET name = ?, item_ids = ?, style = ?, occasion = ?, seasons = ?, rating = ?, favourite = ?, notes = ?, photo_id = ?,
		updated_at = strftime('%Y-%m-%dT%H:%M:%fZ','now') WHERE user_id = ? AND id = ?`,
		o.Name, string(items), o.Style, o.Occasion, csv(o.Seasons), o.Rating, boolInt(o.Favourite), o.Notes, nullI(o.PhotoID), o.UserID, o.ID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteOutfit removes an outfit (the days it was worn stay, without it).
func (s *Store) DeleteOutfit(ctx context.Context, uid, id int64) error {
	o, err := s.GetOutfit(ctx, uid, id)
	if err != nil {
		return err
	}
	if _, err := s.DB.ExecContext(ctx, `DELETE FROM outfits WHERE user_id = ? AND id = ?`, uid, id); err != nil {
		return err
	}
	if o.PhotoID != nil {
		_ = s.DeletePhotoIfUnused(ctx, uid, *o.PhotoID)
	}
	return nil
}

// FindOutfit is a saved outfit with exactly these pieces, if there is one.
func (s *Store) FindOutfit(ctx context.Context, uid int64, list []int64) (*Outfit, error) {
	all, err := s.ListOutfits(ctx, uid)
	if err != nil {
		return nil, err
	}
	for _, o := range all {
		if sameSet(o.ItemIDs, list) {
			return o, nil
		}
	}
	return nil, ErrNotFound
}

func sameSet(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	m := map[int64]bool{}
	for _, x := range a {
		m[x] = true
	}
	for _, x := range b {
		if !m[x] {
			return false
		}
	}
	return true
}
