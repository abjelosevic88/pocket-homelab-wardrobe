package store

import (
	"context"
	"database/sql"
	"encoding/json"
)

const wearCols = `id, user_id, date, planned, outfit_id, item_ids, occasion, event, event_ref, weather, feel, notes, photo_id, source, created_at`

func scanWear(sc interface{ Scan(...any) error }) (*Wear, error) {
	var w Wear
	var planned int
	var outfit, photo sql.NullInt64
	var items, weather string
	if err := sc.Scan(&w.ID, &w.UserID, &w.Date, &planned, &outfit, &items, &w.Occasion, &w.Event, &w.EventRef, &weather, &w.Feel, &w.Notes, &photo, &w.Source, &w.CreatedAt); err != nil {
		return nil, err
	}
	w.Planned, w.ItemIDs = planned == 1, ids(items)
	if outfit.Valid {
		w.OutfitID = &outfit.Int64
	}
	if photo.Valid {
		w.PhotoID = &photo.Int64
	}
	if weather != "" && json.Valid([]byte(weather)) {
		w.Weather = json.RawMessage(weather)
	}
	return &w, nil
}

// ListWears lists a person's worn and planned days from…to (inclusive), oldest first.
func (s *Store) ListWears(ctx context.Context, uid int64, from, to string) ([]*Wear, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT `+wearCols+` FROM wears WHERE user_id = ? AND date >= ? AND date <= ? ORDER BY date, planned, id`, uid, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*Wear{}
	for rows.Next() {
		w, err := scanWear(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

// GetWear returns one of a person's wears.
func (s *Store) GetWear(ctx context.Context, uid, id int64) (*Wear, error) {
	w, err := scanWear(s.DB.QueryRowContext(ctx, `SELECT `+wearCols+` FROM wears WHERE user_id = ? AND id = ?`, uid, id))
	return w, notFound(err)
}

// PlanFor is the plan for a day, if there is one.
func (s *Store) PlanFor(ctx context.Context, uid int64, date string) (*Wear, error) {
	w, err := scanWear(s.DB.QueryRowContext(ctx, `SELECT `+wearCols+` FROM wears WHERE user_id = ? AND date = ? AND planned = 1 ORDER BY id DESC LIMIT 1`, uid, date))
	return w, notFound(err)
}

func (s *Store) writeWearItems(ctx context.Context, tx *sql.Tx, w *Wear) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM wear_items WHERE wear_id = ?`, w.ID); err != nil {
		return err
	}
	for _, id := range w.ItemIDs {
		if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO wear_items (wear_id, item_id, user_id, date, planned) VALUES (?, ?, ?, ?, ?)`, w.ID, id, w.UserID, w.Date, boolInt(w.Planned)); err != nil {
			return err
		}
	}
	return nil
}

func rawOr(r json.RawMessage) string {
	if len(r) == 0 || string(r) == "null" {
		return ""
	}
	return string(r)
}

// CreateWear logs a day's outfit (or a plan for a day).
func (s *Store) CreateWear(ctx context.Context, w *Wear) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	items, _ := json.Marshal(w.ItemIDs)
	res, err := tx.ExecContext(ctx, `INSERT INTO wears (user_id, date, planned, outfit_id, item_ids, occasion, event, event_ref, weather, feel, notes, photo_id, source) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		w.UserID, w.Date, boolInt(w.Planned), nullI(w.OutfitID), string(items), w.Occasion, w.Event, w.EventRef, rawOr(w.Weather), w.Feel, w.Notes, nullI(w.PhotoID), w.Source)
	if err != nil {
		return err
	}
	w.ID, _ = res.LastInsertId()
	if err := s.writeWearItems(ctx, tx, w); err != nil {
		return err
	}
	return tx.Commit()
}

// UpdateWear saves a wear (and turns a plan into a worn day).
func (s *Store) UpdateWear(ctx context.Context, w *Wear) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	items, _ := json.Marshal(w.ItemIDs)
	res, err := tx.ExecContext(ctx, `UPDATE wears SET date = ?, planned = ?, outfit_id = ?, item_ids = ?, occasion = ?, event = ?, event_ref = ?, weather = ?, feel = ?, notes = ?, photo_id = ?
		WHERE user_id = ? AND id = ?`, w.Date, boolInt(w.Planned), nullI(w.OutfitID), string(items), w.Occasion, w.Event, w.EventRef, rawOr(w.Weather), w.Feel, w.Notes, nullI(w.PhotoID), w.UserID, w.ID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	if err := s.writeWearItems(ctx, tx, w); err != nil {
		return err
	}
	return tx.Commit()
}

// DeleteWear removes a wear.
func (s *Store) DeleteWear(ctx context.Context, uid, id int64) error {
	w, err := s.GetWear(ctx, uid, id)
	if err != nil {
		return err
	}
	if _, err := s.DB.ExecContext(ctx, `DELETE FROM wears WHERE user_id = ? AND id = ?`, uid, id); err != nil {
		return err
	}
	if w.PhotoID != nil {
		_ = s.DeletePhotoIfUnused(ctx, uid, *w.PhotoID)
	}
	return nil
}

// LastWorn is, per piece, the day it was last worn (before or on a day).
func (s *Store) LastWorn(ctx context.Context, uid int64, upto string) (map[int64]string, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT item_id, MAX(date) FROM wear_items WHERE user_id = ? AND planned = 0 AND date <= ? GROUP BY item_id`, uid, upto)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]string{}
	for rows.Next() {
		var id int64
		var d string
		if err := rows.Scan(&id, &d); err != nil {
			return nil, err
		}
		out[id] = d
	}
	return out, rows.Err()
}

// WearCounts is, per piece, how often it was worn from a day on.
func (s *Store) WearCounts(ctx context.Context, uid int64, from string) (map[int64]int, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT item_id, COUNT(1) FROM wear_items WHERE user_id = ? AND planned = 0 AND date >= ? GROUP BY item_id`, uid, from)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]int{}
	for rows.Next() {
		var id int64
		var n int
		if err := rows.Scan(&id, &n); err != nil {
			return nil, err
		}
		out[id] = n
	}
	return out, rows.Err()
}

// DaysWorn is how many days from a day on have something logged.
func (s *Store) DaysWorn(ctx context.Context, uid int64, from string) int {
	var n int
	_ = s.DB.QueryRowContext(ctx, `SELECT COUNT(DISTINCT date) FROM wears WHERE user_id = ? AND planned = 0 AND date >= ?`, uid, from).Scan(&n)
	return n
}
