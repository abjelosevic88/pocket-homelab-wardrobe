# API

Everything the web app does goes through `/api/v1`, with a session cookie or `Authorization: Bearer pwdr_…` (Settings → API & shortcuts). Money is in **minor units** (cents) with its currency; days are `YYYY-MM-DD`. Every row belongs to the person signed in; another person's ids answer 404.

## The closet

| Call | What |
|---|---|
| `GET /items` (`?category=top&status=laundry&q=navy&gone=1`) | Pieces with `wears`, `last_worn`, `per_wear` (price ÷ wears), `photo_url`/`thumb_url`; `counts` per category and `states` |
| `POST /items` | `{"kind": "jeans", "colors": [{"hex": "#2c3d58"}], "price": 8900, "photo_id": 3}` — kind sets the category, warmth and formality unless given; colour names come from the hex |
| `GET /items/{id}` | The piece, the days worn, its outfits, `similar` pieces and what it `goes_with` best |
| `PUT /items/{id}` / `DELETE /items/{id}` | Change (only the fields sent) / delete |
| `POST /items/status` | `{"ids": [1,2], "status": "laundry"}`, or `{"from": "laundry", "status": "ready"}` for “all washed”. States: `ready laundry repair lent stored` |
| `POST /items/{id}/gone` | `{"reason": "donated|sold|thrown|lost", "sold_for": 2000}`; `""` brings it back |
| `POST /items/{id}/task` | `{"what": "tailor|dry_clean|repair|return|sell", "due": "…"}` → Pocket Tasks |
| `POST /photos` | multipart `file` (+ optional `thumb`, `kind`) → `{"id", "url", "thumb_url"}`; `GET /photos/{id}?thumb=1` |

## Outfits and ideas

| Call | What |
|---|---|
| `GET/POST /outfits`, `GET/PUT/DELETE /outfits/{id}` | Saved outfits (`item_ids`, `occasion`, `style`, `rating`); the page also has the score and a `share` picture link |
| `POST /suggest` | `{"date", "occasion", "style", "include": 12, "shuffle": 1, "n": 6}` (or `t_min`/`t_max`/`rain`/`season` instead of the forecast) → scored outfits with `reasons` and `warnings` |
| `POST /rate` | `{"item_ids": […], "occasion", "style", "weather": true}` → the score of pieces put together by hand |
| `GET /styles` | Styles, occasions, kinds, categories, colour families, patterns |
| `GET /today` (`?date&occasion&style&shuffle`) | Weather, events, training, outing, the plan, what was worn, ideas and tips; tomorrow, the laundry and a piece waiting to be worn |
| `GET /wears?from&to`, `POST /wears`, `PUT/DELETE /wears/{id}` | What was worn; `"planned": true` for a day ahead (one plan a day); `PUT {"planned": false}` turns a plan into a worn day |

## Shopping and packing

| Call | What |
|---|---|
| `GET /gaps` (`?style=classic`) | Staples missing for your styles: drawn colour, `unlocks` (new outfits) and example outfits (`-1` = the idea), shop links |
| `GET/POST /ideas`, `PUT/DELETE /ideas/{id}` | Kept ideas; `POST /ideas/{id}/wishlist` → Pocket Wishlist; `POST /ideas/{id}/bought` → a piece |
| `POST /check` | `{"kind": "sweater", "color_hex": "#22304f"}` → `verdict` (`great good think skip`), similar pieces, outfits it makes |
| `POST /preview` | `{"url": "…"}` → a shop page's title, image and price (through Pocket Wishlist when connected) |
| `GET /purchases` | Clothing from Pocket Money and Pocket Parcels, with `added` when it is in the closet |
| `GET/POST /packs`, `GET/PUT/DELETE /packs/{id}` | Packing lists; `POST /packs/{id}/fill` picks a capsule; `POST /packs/{id}/note` → Pocket Notes; `GET /trips` from Pocket Travel |

## AI (only when asked)

`POST /ai/tag` (`{"photo_id", "many"}`), `/ai/style-me`, `/ai/rate-look`, `/ai/find-style`, `/ai/colours`, `/ai/shopping`, `/ai/check`, `/packs/{id}/ai`. Pocket AI through Core (photos from Pocket AI 0.87), or your own OpenAI-compatible key.

## For the rest of the box

`GET /summary` (Homepage), `GET /day/{date}` (Pocket Day), `GET /search?q=` (Pocket Search), `GET /stats?days=90`, `GET /export.json` / `.csv`, `POST /import`.

Another app acting for you with a `pcx_` token reaches only: `wardrobe:read` — `GET summary, today, stats, items, items/{id}, outfits, outfits/{id}, wears, packs, packs/{id}, ideas, styles, search, day/{date}, photos/{id}` and `POST /suggest`; `wardrobe:write` — `POST /items`, `/outfits`, `/wears` and `PUT`/`DELETE` of what it added itself.
