package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
)

// csv joins a list for a comma-separated column.
func csv(list []string) string {
	out := []string{}
	for _, v := range list {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return strings.Join(out, ",")
}

func split(v string) []string {
	out := []string{}
	for _, p := range strings.Split(v, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func ids(raw string) []int64 {
	out := []int64{}
	_ = json.Unmarshal([]byte(raw), &out)
	return out
}

func nullS(v string) sql.NullString {
	return sql.NullString{String: v, Valid: v != ""}
}

const itemCols = `i.id, i.user_id, i.name, i.category, i.kind, i.colors, i.pattern, i.material, i.fit, i.seasons, i.warmth, i.formality, i.styles, i.rain,
	i.brand, i.size, i.price, i.currency, i.bought_on, i.shop, i.care, i.notes, i.status, i.status_note, i.status_at, i.favourite, i.gone, i.gone_on, i.sold_for,
	i.photo_id, i.source, i.source_ref, i.created_at, i.updated_at,
	(SELECT COUNT(1) FROM wear_items w WHERE w.item_id = i.id AND w.planned = 0),
	COALESCE((SELECT MAX(w.date) FROM wear_items w WHERE w.item_id = i.id AND w.planned = 0), '')`

func scanItem(sc interface{ Scan(...any) error }) (*Item, error) {
	var it Item
	var colors, seasons, styles string
	var price, sold, photo sql.NullInt64
	var rain, fav int
	if err := sc.Scan(&it.ID, &it.UserID, &it.Name, &it.Category, &it.Kind, &colors, &it.Pattern, &it.Material, &it.Fit, &seasons, &it.Warmth, &it.Formality, &styles, &rain,
		&it.Brand, &it.Size, &price, &it.Currency, &it.BoughtOn, &it.Shop, &it.Care, &it.Notes, &it.Status, &it.StatusNote, &it.StatusAt, &fav, &it.Gone, &it.GoneOn, &sold,
		&photo, &it.Source, &it.SourceRef, &it.CreatedAt, &it.UpdatedAt, &it.Wears, &it.LastWorn); err != nil {
		return nil, err
	}
	_ = json.Unmarshal([]byte(colors), &it.Colors)
	if it.Colors == nil {
		it.Colors = []Color{}
	}
	it.Seasons, it.Styles = split(seasons), split(styles)
	it.Rain, it.Favourite = rain == 1, fav == 1
	if price.Valid {
		it.Price = &price.Int64
		if it.Wears > 0 {
			v := price.Int64 / int64(it.Wears)
			it.PerWear = &v
		}
	}
	if sold.Valid {
		it.SoldFor = &sold.Int64
	}
	if photo.Valid {
		it.PhotoID = &photo.Int64
	}
	return &it, nil
}

// ItemFilter narrows the closet.
type ItemFilter struct {
	Category string
	Status   string
	Q        string
	Gone     bool // the pieces you no longer have (instead of the ones you do)
	Source   string
	Ref      string
}

// ListItems lists a person's pieces.
func (s *Store) ListItems(ctx context.Context, uid int64, f ItemFilter) ([]*Item, error) {
	q := `SELECT ` + itemCols + ` FROM items i WHERE i.user_id = ?`
	args := []any{uid}
	if f.Gone {
		q += ` AND i.gone != ''`
	} else {
		q += ` AND i.gone = ''`
	}
	if f.Category != "" {
		q += ` AND i.category = ?`
		args = append(args, f.Category)
	}
	if f.Status != "" {
		q += ` AND i.status = ?`
		args = append(args, f.Status)
	}
	if f.Source != "" {
		q += ` AND i.source = ?`
		args = append(args, f.Source)
	}
	if f.Ref != "" {
		q += ` AND i.source_ref = ?`
		args = append(args, f.Ref)
	}
	if f.Q != "" {
		like := "%" + strings.ToLower(f.Q) + "%"
		q += ` AND (lower(i.name) LIKE ? OR lower(i.brand) LIKE ? OR lower(i.kind) LIKE ? OR lower(i.colors) LIKE ? OR lower(i.notes) LIKE ? OR lower(i.material) LIKE ?)`
		args = append(args, like, like, like, like, like, like)
	}
	q += ` ORDER BY i.category, i.favourite DESC, i.name COLLATE NOCASE`
	rows, err := s.DB.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*Item{}
	for rows.Next() {
		it, err := scanItem(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

// GetItem returns one of a person's pieces.
func (s *Store) GetItem(ctx context.Context, uid, id int64) (*Item, error) {
	it, err := scanItem(s.DB.QueryRowContext(ctx, `SELECT `+itemCols+` FROM items i WHERE i.user_id = ? AND i.id = ?`, uid, id))
	return it, notFound(err)
}

// ItemsByIDs returns the person's pieces with these ids (others are left out).
func (s *Store) ItemsByIDs(ctx context.Context, uid int64, list []int64) (map[int64]*Item, error) {
	out := map[int64]*Item{}
	if len(list) == 0 {
		return out, nil
	}
	ph := strings.TrimSuffix(strings.Repeat("?,", len(list)), ",")
	args := []any{uid}
	for _, id := range list {
		args = append(args, id)
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT `+itemCols+` FROM items i WHERE i.user_id = ? AND i.id IN (`+ph+`)`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		it, err := scanItem(rows)
		if err != nil {
			return nil, err
		}
		out[it.ID] = it
	}
	return out, rows.Err()
}

// OwnIDs keeps the ids that are the person's own pieces, in order.
func (s *Store) OwnIDs(ctx context.Context, uid int64, list []int64) ([]int64, error) {
	m, err := s.ItemsByIDs(ctx, uid, list)
	if err != nil {
		return nil, err
	}
	out := []int64{}
	seen := map[int64]bool{}
	for _, id := range list {
		if m[id] != nil && !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out, nil
}

func itemArgs(it *Item) []any {
	colors, _ := json.Marshal(it.Colors)
	return []any{it.Name, it.Category, it.Kind, string(colors), it.Pattern, it.Material, it.Fit, csv(it.Seasons), it.Warmth, it.Formality, csv(it.Styles), boolInt(it.Rain),
		it.Brand, it.Size, nullI64(it.Price), it.Currency, it.BoughtOn, it.Shop, it.Care, it.Notes, it.Status, it.StatusNote, it.StatusAt, boolInt(it.Favourite), it.Gone, it.GoneOn, nullI64(it.SoldFor),
		nullI(it.PhotoID), it.Source, it.SourceRef}
}

func nullI64(p *int64) any {
	if p == nil {
		return nil
	}
	return *p
}

// CreateItem adds a piece.
func (s *Store) CreateItem(ctx context.Context, it *Item) error {
	if it.Status == "" {
		it.Status = "ready"
	}
	if it.Pattern == "" {
		it.Pattern = "plain"
	}
	res, err := s.DB.ExecContext(ctx, `INSERT INTO items (user_id, name, category, kind, colors, pattern, material, fit, seasons, warmth, formality, styles, rain,
		brand, size, price, currency, bought_on, shop, care, notes, status, status_note, status_at, favourite, gone, gone_on, sold_for, photo_id, source, source_ref)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, append([]any{it.UserID}, itemArgs(it)...)...)
	if err != nil {
		return err
	}
	it.ID, _ = res.LastInsertId()
	return nil
}

// UpdateItem saves a piece.
func (s *Store) UpdateItem(ctx context.Context, it *Item) error {
	res, err := s.DB.ExecContext(ctx, `UPDATE items SET name = ?, category = ?, kind = ?, colors = ?, pattern = ?, material = ?, fit = ?, seasons = ?, warmth = ?, formality = ?, styles = ?, rain = ?,
		brand = ?, size = ?, price = ?, currency = ?, bought_on = ?, shop = ?, care = ?, notes = ?, status = ?, status_note = ?, status_at = ?, favourite = ?, gone = ?, gone_on = ?, sold_for = ?,
		photo_id = ?, source = ?, source_ref = ?, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ','now') WHERE user_id = ? AND id = ?`, append(itemArgs(it), it.UserID, it.ID)...)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// SetItemStatus moves pieces to the laundry, back in the closet and so on.
func (s *Store) SetItemStatus(ctx context.Context, uid int64, list []int64, status, note string) (int, error) {
	n := 0
	for _, id := range list {
		res, err := s.DB.ExecContext(ctx, `UPDATE items SET status = ?, status_note = ?, status_at = ?, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ','now') WHERE user_id = ? AND id = ?`, status, note, Today(), uid, id)
		if err != nil {
			return n, err
		}
		if k, _ := res.RowsAffected(); k > 0 {
			n++
		}
	}
	return n, nil
}

// DeleteItem removes a piece for good (and its photo).
func (s *Store) DeleteItem(ctx context.Context, uid, id int64) error {
	it, err := s.GetItem(ctx, uid, id)
	if err != nil {
		return err
	}
	if _, err := s.DB.ExecContext(ctx, `DELETE FROM items WHERE user_id = ? AND id = ?`, uid, id); err != nil {
		return err
	}
	if it.PhotoID != nil {
		_ = s.DeletePhotoIfUnused(ctx, uid, *it.PhotoID)
	}
	return nil
}

// ItemWears lists the days a piece was worn, newest first.
func (s *Store) ItemWears(ctx context.Context, uid, id int64, limit int) ([]string, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT date FROM wear_items WHERE user_id = ? AND item_id = ? AND planned = 0 ORDER BY date DESC LIMIT ?`, uid, id, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}
