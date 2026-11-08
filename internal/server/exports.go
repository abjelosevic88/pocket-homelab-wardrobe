package server

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/abjelosevic88/pocket-homelab-wardrobe/internal/store"
)

// exportDoc is everything of one person, to keep or to bring to another box.
type exportDoc struct {
	App        string          `json:"app"`
	Version    string          `json:"version"`
	ExportedAt string          `json:"exported_at"`
	Items      []*store.Item   `json:"items"`
	Outfits    []*store.Outfit `json:"outfits"`
	Wears      []*store.Wear   `json:"wears"`
	Ideas      []*store.Idea   `json:"ideas"`
	Packs      []*store.Pack   `json:"packs"`
	Photos     []exportPhoto   `json:"photos"`
}

type exportPhoto struct {
	ID    int64  `json:"id"`
	Kind  string `json:"kind"`
	Mime  string `json:"mime"`
	Data  []byte `json:"data"` // base64 in JSON
	Thumb []byte `json:"thumb,omitempty"`
}

func (s *Server) export(ctx context.Context, uid int64, photos bool) (exportDoc, error) {
	doc := exportDoc{App: "pocket-wardrobe", Version: Version, ExportedAt: time.Now().UTC().Format(time.RFC3339), Photos: []exportPhoto{}}
	have, err := s.store.ListItems(ctx, uid, store.ItemFilter{})
	if err != nil {
		return doc, err
	}
	gone, err := s.store.ListItems(ctx, uid, store.ItemFilter{Gone: true})
	if err != nil {
		return doc, err
	}
	doc.Items = append(have, gone...)
	if doc.Outfits, err = s.store.ListOutfits(ctx, uid); err != nil {
		return doc, err
	}
	if doc.Wears, err = s.store.ListWears(ctx, uid, "0000-01-01", "9999-12-31"); err != nil {
		return doc, err
	}
	if doc.Ideas, err = s.store.ListIdeas(ctx, uid, ""); err != nil {
		return doc, err
	}
	if doc.Packs, err = s.store.ListPacks(ctx, uid); err != nil {
		return doc, err
	}
	if photos {
		seen := map[int64]bool{}
		add := func(id *int64) {
			if id == nil || seen[*id] {
				return
			}
			seen[*id] = true
			if big, err := s.store.GetPhoto(ctx, uid, *id, false); err == nil {
				ep := exportPhoto{ID: big.ID, Kind: big.Kind, Mime: big.Mime, Data: big.Data}
				if small, err := s.store.GetPhoto(ctx, uid, *id, true); err == nil && len(small.Data) != len(big.Data) {
					ep.Thumb = small.Data
				}
				doc.Photos = append(doc.Photos, ep)
			}
		}
		for _, x := range doc.Items {
			add(x.PhotoID)
		}
		for _, x := range doc.Outfits {
			add(x.PhotoID)
		}
		for _, x := range doc.Wears {
			add(x.PhotoID)
		}
		for _, x := range doc.Ideas {
			add(x.PhotoID)
		}
	}
	return doc, nil
}

func (s *Server) handleExportJSON(w http.ResponseWriter, r *http.Request) {
	doc, err := s.export(r.Context(), actorID(r.Context()), !queryBool(r, "no_photos"))
	if err != nil {
		s.fail(w, err, "export")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="pocket-wardrobe-%s.json"`, time.Now().Format("20060102")))
	_ = json.NewEncoder(w).Encode(doc)
}

// handleExportCSV is the closet as a spreadsheet: one piece a row.
func (s *Server) handleExportCSV(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	list, err := s.store.ListItems(ctx, actorID(ctx), store.ItemFilter{})
	if err != nil {
		s.fail(w, err, "export")
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="pocket-wardrobe-%s.csv"`, time.Now().Format("20060102")))
	_, _ = w.Write([]byte("\ufeff"))
	cw := csv.NewWriter(w)
	_ = cw.Write([]string{"name", "category", "kind", "colours", "pattern", "material", "brand", "size", "price", "currency", "bought_on", "shop", "seasons", "warmth", "formality", "status", "wears", "last_worn", "cost_per_wear", "notes"})
	money := func(p *int64) string {
		if p == nil {
			return ""
		}
		return fmt.Sprintf("%.2f", float64(*p)/100)
	}
	for _, it := range list {
		var cs []string
		for _, c := range it.Colors {
			cs = append(cs, c.Name)
		}
		_ = cw.Write([]string{it.Name, it.Category, it.Kind, strings.Join(cs, " + "), it.Pattern, it.Material, it.Brand, it.Size, money(it.Price), it.Currency, it.BoughtOn, it.Shop,
			strings.Join(it.Seasons, " "), fmt.Sprint(it.Warmth), fmt.Sprint(it.Formality), it.Status, fmt.Sprint(it.Wears), it.LastWorn, money(it.PerWear), it.Notes})
	}
	cw.Flush()
}

// handleImport takes a Pocket Wardrobe export (again and again: nothing is doubled).
func (s *Server) handleImport(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	var data []byte
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/") {
		r.Body = http.MaxBytesReader(w, r.Body, 512<<20)
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			writeErr(w, http.StatusBadRequest, "the file is too big")
			return
		}
		f, _, err := r.FormFile("file")
		if err != nil {
			writeErr(w, http.StatusBadRequest, "no file")
			return
		}
		defer f.Close()
		if data, err = io.ReadAll(f); err != nil {
			writeErr(w, http.StatusBadRequest, "could not read the file")
			return
		}
	} else {
		b, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 512<<20))
		if err != nil {
			writeErr(w, http.StatusBadRequest, "the file is too big")
			return
		}
		data = b
	}
	data = bytes.TrimPrefix(data, []byte("\ufeff"))
	var doc exportDoc
	if err := json.Unmarshal(data, &doc); err != nil || doc.App != "pocket-wardrobe" {
		writeErr(w, http.StatusBadRequest, "that is not a Pocket Wardrobe export")
		return
	}
	res, err := s.importPocket(ctx, uid, doc)
	if err != nil {
		s.fail(w, err, "import")
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// importPocket brings back an export. Each piece remembers where it came
// from (source + ref), so a second import finds it instead of doubling it.
func (s *Server) importPocket(ctx context.Context, uid int64, doc exportDoc) (map[string]any, error) {
	added := map[string]int{}
	photos := map[int64]exportPhoto{}
	for _, p := range doc.Photos {
		photos[p.ID] = p
	}
	newPhoto := map[int64]int64{}
	photo := func(old *int64) *int64 {
		if old == nil {
			return nil
		}
		if id, ok := newPhoto[*old]; ok {
			return &id
		}
		p, ok := photos[*old]
		if !ok || len(p.Data) == 0 {
			return nil
		}
		np := &store.Photo{UserID: uid, Kind: firstNonEmpty(p.Kind, "item"), Mime: firstNonEmpty(p.Mime, "image/jpeg"), Data: p.Data, Thumb: p.Thumb}
		if err := s.store.CreatePhoto(ctx, np); err != nil {
			return nil
		}
		newPhoto[*old] = np.ID
		added["photos"]++
		return &np.ID
	}
	itemIDs := map[int64]int64{}
	for _, it := range doc.Items {
		src, ref := it.Source, it.SourceRef
		if ref == "" {
			src, ref = "import", fmt.Sprintf("item:%d:%s", it.ID, it.CreatedAt)
		}
		if have, _ := s.store.ListItems(ctx, uid, store.ItemFilter{Source: src, Ref: ref}); len(have) > 0 {
			itemIDs[it.ID] = have[0].ID
			continue
		}
		if have, _ := s.store.ListItems(ctx, uid, store.ItemFilter{Source: src, Ref: ref, Gone: true}); len(have) > 0 {
			itemIDs[it.ID] = have[0].ID
			continue
		}
		old := it.ID
		it.UserID, it.Source, it.SourceRef = uid, src, ref
		it.PhotoID = photo(it.PhotoID)
		if !oneOf(it.Status, store.ItemStates) {
			it.Status = "ready"
		}
		if err := s.store.CreateItem(ctx, it); err != nil {
			return nil, err
		}
		itemIDs[old] = it.ID
		added["items"]++
	}
	mapIDs := func(list []int64) []int64 {
		out := []int64{}
		for _, id := range list {
			if n, ok := itemIDs[id]; ok {
				out = append(out, n)
			}
		}
		return out
	}
	outfitIDs := map[int64]int64{}
	for _, o := range doc.Outfits {
		ids := mapIDs(o.ItemIDs)
		if len(ids) == 0 {
			continue
		}
		if have, err := s.store.FindOutfit(ctx, uid, ids); err == nil {
			outfitIDs[o.ID] = have.ID
			continue
		}
		old := o.ID
		o.UserID, o.ItemIDs, o.PhotoID = uid, ids, photo(o.PhotoID)
		if err := s.store.CreateOutfit(ctx, o); err != nil {
			return nil, err
		}
		outfitIDs[old] = o.ID
		added["outfits"]++
	}
	for _, x := range doc.Wears {
		ids := mapIDs(x.ItemIDs)
		if len(ids) == 0 {
			continue
		}
		dup := false
		if list, _ := s.store.ListWears(ctx, uid, x.Date, x.Date); len(list) > 0 {
			for _, y := range list {
				if y.Planned == x.Planned && len(y.ItemIDs) == len(ids) {
					dup = true
				}
			}
		}
		if dup {
			continue
		}
		x.UserID, x.ItemIDs, x.PhotoID = uid, ids, photo(x.PhotoID)
		if x.OutfitID != nil {
			if n, ok := outfitIDs[*x.OutfitID]; ok {
				x.OutfitID = &n
			} else {
				x.OutfitID = nil
			}
		}
		if err := s.store.CreateWear(ctx, x); err != nil {
			return nil, err
		}
		added["wears"]++
	}
	ideas, _ := s.store.ListIdeas(ctx, uid, "")
	for _, i := range doc.Ideas {
		dup := false
		for _, h := range ideas {
			if h.Title == i.Title && h.Kind == i.Kind {
				dup = true
			}
		}
		if dup {
			continue
		}
		i.UserID, i.PhotoID = uid, photo(i.PhotoID)
		if err := s.store.CreateIdea(ctx, i); err != nil {
			return nil, err
		}
		added["ideas"]++
	}
	packs, _ := s.store.ListPacks(ctx, uid)
	for _, p := range doc.Packs {
		dup := false
		for _, h := range packs {
			if h.Name == p.Name && h.StartDate == p.StartDate {
				dup = true
			}
		}
		if dup {
			continue
		}
		var items []store.PackItem
		for _, it := range p.Items {
			if n, ok := itemIDs[it.ItemID]; ok {
				it.ItemID = n
				items = append(items, it)
			}
		}
		p.UserID, p.Items = uid, items
		if p.Items == nil {
			p.Items = []store.PackItem{}
		}
		if err := s.store.CreatePack(ctx, p); err != nil {
			return nil, err
		}
		added["packs"]++
	}
	return map[string]any{"added": added}, nil
}
