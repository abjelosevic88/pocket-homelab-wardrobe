// Package style is the outfit engine: what goes with what, for a style, an
// occasion and the weather, without any AI. Colours, warmth, formality and
// patterns are scored by rules a person can read back ("navy and white
// always work", "warm enough for 6°C").
package style

// Category is where a piece sits in an outfit.
type Category struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Plural string `json:"plural"`
}

// Categories in the order the closet shows them.
var Categories = []Category{
	{"top", "Top", "Tops"},
	{"knit", "Knit or layer", "Knits and layers"},
	{"bottom", "Bottom", "Trousers, jeans and skirts"},
	{"dress", "Dress", "Dresses and jumpsuits"},
	{"outerwear", "Jacket or coat", "Jackets and coats"},
	{"shoes", "Shoes", "Shoes"},
	{"bag", "Bag", "Bags"},
	{"accessory", "Accessory", "Accessories"},
	{"active", "Sportswear", "Sportswear"},
	{"other", "Other", "Other"},
}

// Kind is one sort of piece with its usual warmth and formality; the
// shape is what the drawing of it looks like.
type Kind struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	Category  string `json:"category"`
	Warmth    int    `json:"warmth"`
	Formality int    `json:"formality"`
	Shape     string `json:"shape"`
	Rain      bool   `json:"rain,omitempty"`
}

// Kinds a piece can be.
var Kinds = []Kind{
	{"tshirt", "T-shirt", "top", 1, 1, "tshirt", false},
	{"tank", "Vest top", "top", 1, 1, "tank", false},
	{"polo", "Polo shirt", "top", 1, 2, "polo", false},
	{"shirt", "Shirt", "top", 2, 3, "shirt", false},
	{"blouse", "Blouse", "top", 1, 3, "blouse", false},
	{"longsleeve", "Long-sleeve top", "top", 2, 2, "longsleeve", false},
	{"sweater", "Jumper", "knit", 3, 2, "sweater", false},
	{"turtleneck", "Turtleneck", "knit", 3, 3, "turtleneck", false},
	{"cardigan", "Cardigan", "knit", 3, 2, "cardigan", false},
	{"hoodie", "Hoodie", "knit", 3, 1, "hoodie", false},
	{"sweatshirt", "Sweatshirt", "knit", 3, 1, "sweater", false},
	{"fleece", "Fleece", "knit", 3, 1, "hoodie", false},
	{"jeans", "Jeans", "bottom", 3, 2, "trousers", false},
	{"chinos", "Chinos", "bottom", 2, 3, "trousers", false},
	{"trousers", "Smart trousers", "bottom", 2, 4, "trousers", false},
	{"cargo", "Cargo trousers", "bottom", 3, 1, "trousers", false},
	{"joggers", "Joggers", "bottom", 2, 1, "joggers", false},
	{"shorts", "Shorts", "bottom", 1, 1, "shorts", false},
	{"skirt", "Skirt", "bottom", 1, 3, "skirt", false},
	{"leggings", "Leggings", "bottom", 2, 1, "joggers", false},
	{"dress", "Dress", "dress", 1, 3, "dress", false},
	{"evening_dress", "Evening dress", "dress", 1, 5, "gown", false},
	{"jumpsuit", "Jumpsuit", "dress", 2, 3, "jumpsuit", false},
	{"blazer", "Blazer", "outerwear", 2, 4, "blazer", false},
	{"jacket", "Light jacket", "outerwear", 2, 2, "jacket", false},
	{"denim_jacket", "Denim jacket", "outerwear", 2, 1, "jacket", false},
	{"leather_jacket", "Leather jacket", "outerwear", 3, 2, "jacket", false},
	{"raincoat", "Rain jacket", "outerwear", 2, 1, "parka", true},
	{"trench", "Trench coat", "outerwear", 3, 3, "coat", true},
	{"coat", "Wool coat", "outerwear", 4, 4, "coat", false},
	{"puffer", "Puffer jacket", "outerwear", 5, 1, "puffer", false},
	{"parka", "Parka", "outerwear", 5, 1, "parka", true},
	{"sneakers", "Trainers", "shoes", 1, 1, "sneaker", false},
	{"loafers", "Loafers", "shoes", 1, 3, "loafer", false},
	{"smart_shoes", "Smart shoes", "shoes", 1, 4, "oxford", false},
	{"boots", "Ankle boots", "shoes", 2, 3, "boot", false},
	{"winter_boots", "Winter boots", "shoes", 3, 1, "boot", true},
	{"heels", "Heels", "shoes", 1, 4, "heel", false},
	{"flats", "Flats", "shoes", 1, 3, "flat", false},
	{"sandals", "Sandals", "shoes", 1, 1, "sandal", false},
	{"running_shoes", "Running shoes", "shoes", 1, 1, "sneaker", false},
	{"bag", "Bag", "bag", 0, 3, "bag", false},
	{"backpack", "Backpack", "bag", 0, 1, "backpack", false},
	{"tote", "Tote bag", "bag", 0, 2, "tote", false},
	{"belt", "Belt", "accessory", 0, 3, "belt", false},
	{"scarf", "Scarf", "accessory", 2, 2, "scarf", false},
	{"hat", "Hat", "accessory", 1, 2, "hat", false},
	{"cap", "Cap", "accessory", 0, 1, "cap", false},
	{"beanie", "Beanie", "accessory", 2, 1, "beanie", false},
	{"tie", "Tie", "accessory", 0, 5, "tie", false},
	{"watch", "Watch", "accessory", 0, 3, "watch", false},
	{"sunglasses", "Sunglasses", "accessory", 0, 2, "sunglasses", false},
	{"jewellery", "Jewellery", "accessory", 0, 3, "jewellery", false},
	{"gloves", "Gloves", "accessory", 2, 2, "gloves", false},
	{"sports_top", "Sports top", "active", 1, 1, "tshirt", false},
	{"sports_bottom", "Sports shorts or tights", "active", 1, 1, "shorts", false},
	{"swimwear", "Swimwear", "other", 0, 1, "swim", false},
	{"sleepwear", "Pyjamas", "other", 1, 1, "longsleeve", false},
	{"other", "Something else", "other", 1, 2, "tshirt", false},
}

// KindByID finds a kind ("" when unknown).
func KindByID(id string) (Kind, bool) {
	for _, k := range Kinds {
		if k.ID == id {
			return k, true
		}
	}
	return Kind{}, false
}

// Patterns a piece can have.
var Patterns = []string{"plain", "stripes", "checks", "dots", "floral", "print", "other"}

// Seasons of the year.
var Seasons = []string{"spring", "summer", "autumn", "winter"}

// Style is a way of dressing a person can pick.
type Style struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Palette     []string `json:"palette"` // colour families
	Prefers     []string `json:"prefers"` // kinds
	Avoids      []string `json:"avoids"`
	FormalLo    int      `json:"formal_lo"`
	FormalHi    int      `json:"formal_hi"`
	Accents     int      `json:"accents"` // colours other than neutrals that still look right
	Patterns    bool     `json:"patterns"`
	Staples     []Staple `json:"staples"`
}

// Staple is a piece a style is built on.
type Staple struct {
	Kind   string `json:"kind"`
	Family string `json:"family"`
	Title  string `json:"title"`
	Why    string `json:"why"`
}

// Styles to choose from.
var Styles = []Style{
	{ID: "smart_casual", Name: "Smart casual", Description: "Polished but relaxed: chinos and good jeans, shirts and fine knits, clean trainers or loafers.",
		Palette: []string{"navy", "white", "grey", "camel", "light blue", "beige", "denim", "brown"}, Prefers: []string{"chinos", "shirt", "sweater", "loafers", "sneakers", "jeans", "blazer", "polo", "boots", "coat"},
		Avoids: []string{"joggers", "hoodie", "sports_top"}, FormalLo: 2, FormalHi: 4, Accents: 1, Patterns: true,
		Staples: []Staple{
			{"sneakers", "white", "White leather trainers", "Go with jeans, chinos and even a suit on a relaxed day."},
			{"chinos", "navy", "Navy chinos", "Smarter than jeans, easier than trousers; every light top goes with them."},
			{"shirt", "light blue", "Light blue shirt", "The shirt that goes with navy, grey, beige and denim."},
			{"sweater", "grey", "Grey crew-neck jumper", "Over a shirt or a T-shirt, autumn to spring."},
			{"jeans", "denim", "Dark straight jeans", "The base of half the casual outfits there are."},
			{"coat", "camel", "Camel coat", "Warm, and it makes anything under it look put together."},
			{"loafers", "brown", "Brown loafers", "Between trainers and smart shoes: for chinos, jeans, dresses."},
			{"blazer", "navy", "Navy blazer", "Over a T-shirt or a shirt, it makes jeans fit for work and dinner."},
		}},
	{ID: "classic", Name: "Classic", Description: "Timeless tailoring and quiet colours that never date: blazers, trench coats, crisp shirts.",
		Palette: []string{"navy", "white", "camel", "grey", "black", "burgundy", "beige", "cream"}, Prefers: []string{"blazer", "shirt", "trousers", "trench", "loafers", "smart_shoes", "coat", "turtleneck", "blouse", "skirt"},
		Avoids: []string{"hoodie", "joggers", "cargo", "cap"}, FormalLo: 3, FormalHi: 5, Accents: 1, Patterns: false,
		Staples: []Staple{
			{"trench", "beige", "Beige trench coat", "Spring and autumn over everything, rain or not."},
			{"blazer", "navy", "Navy blazer", "The one jacket that goes with almost every trouser and skirt."},
			{"shirt", "white", "Crisp white shirt", "Goes with every colour you own."},
			{"trousers", "grey", "Grey wool trousers", "Smart, and they go with navy, white, camel and black."},
			{"smart_shoes", "black", "Black smart shoes", "For meetings, weddings and dinners."},
			{"turtleneck", "black", "Black fine-knit turtleneck", "Under a blazer or a coat, instead of a shirt."},
			{"coat", "camel", "Camel wool coat", "The classic winter coat."},
		}},
	{ID: "minimal", Name: "Minimal", Description: "Clean lines and few colours: black, white, grey, beige and navy, plain fabrics, good fit.",
		Palette: []string{"black", "white", "grey", "charcoal", "beige", "cream", "navy"}, Prefers: []string{"tshirt", "trousers", "sweater", "coat", "sneakers", "boots", "turtleneck", "dress"},
		Avoids: []string{}, FormalLo: 1, FormalHi: 4, Accents: 0, Patterns: false,
		Staples: []Staple{
			{"tshirt", "white", "Plain white T-shirt", "Under everything, on its own in summer."},
			{"tshirt", "black", "Plain black T-shirt", "The other half of a minimal wardrobe."},
			{"trousers", "black", "Black straight trousers", "Smart or casual depending on the shoes."},
			{"coat", "charcoal", "Charcoal coat", "Plain and long: the minimal winter layer."},
			{"sweater", "beige", "Beige knit", "A soft colour that keeps black and white from looking harsh."},
			{"sneakers", "white", "White minimal trainers", "Clean shoes for every casual day."},
		}},
	{ID: "casual", Name: "Relaxed casual", Description: "Comfortable everyday clothes: jeans, T-shirts, hoodies and trainers.",
		Palette: []string{"denim", "white", "grey", "navy", "black", "olive", "green", "blue"}, Prefers: []string{"jeans", "tshirt", "hoodie", "sweatshirt", "sneakers", "denim_jacket", "shorts", "longsleeve"},
		Avoids: []string{"smart_shoes", "tie", "evening_dress"}, FormalLo: 1, FormalHi: 2, Accents: 2, Patterns: true,
		Staples: []Staple{
			{"jeans", "denim", "Mid-blue jeans", "Go with every T-shirt and hoodie you own."},
			{"tshirt", "white", "White T-shirt", "The easiest top there is."},
			{"hoodie", "grey", "Grey hoodie", "On its own or under a jacket."},
			{"sneakers", "white", "White trainers", "For every casual outfit."},
			{"denim_jacket", "denim", "Denim jacket", "The spring and autumn layer for casual days."},
		}},
	{ID: "street", Name: "Streetwear", Description: "Hoodies, sweatshirts, cargo trousers, big trainers and caps; bold prints welcome.",
		Palette: []string{"black", "white", "grey", "olive", "charcoal", "beige", "orange", "red"}, Prefers: []string{"hoodie", "sweatshirt", "cargo", "joggers", "sneakers", "cap", "puffer", "tshirt", "jacket"},
		Avoids: []string{"smart_shoes", "tie", "blouse", "blazer"}, FormalLo: 1, FormalHi: 2, Accents: 2, Patterns: true,
		Staples: []Staple{
			{"hoodie", "black", "Black heavyweight hoodie", "The centre of most street outfits."},
			{"cargo", "olive", "Olive cargo trousers", "Relaxed, practical, and good with black and grey."},
			{"sneakers", "white", "Chunky trainers", "Shoes are half the look."},
			{"puffer", "black", "Black puffer", "Winter on the street."},
			{"cap", "black", "Black cap", "Finishes the look."},
		}},
	{ID: "sporty", Name: "Sporty", Description: "Sportswear for every day: tracksuits, leggings, running shoes, fleeces.",
		Palette: []string{"black", "grey", "white", "navy", "blue", "green"}, Prefers: []string{"sports_top", "sports_bottom", "leggings", "joggers", "running_shoes", "sneakers", "hoodie", "fleece", "cap"},
		Avoids: []string{"smart_shoes", "heels", "blazer", "tie"}, FormalLo: 1, FormalHi: 1, Accents: 2, Patterns: true,
		Staples: []Staple{
			{"running_shoes", "grey", "Running shoes", "For training and long days on your feet."},
			{"leggings", "black", "Black leggings or tights", "Training, and under a long top for the day."},
			{"sports_top", "black", "Breathable sports top", "Dries fast; for the gym and runs."},
			{"fleece", "navy", "Fleece mid-layer", "Warm on cool mornings, light in a bag."},
		}},
	{ID: "business", Name: "Business", Description: "Suits, blazers and smart trousers, shirts and smart shoes: for offices and meetings.",
		Palette: []string{"navy", "grey", "charcoal", "white", "light blue", "black", "burgundy"}, Prefers: []string{"blazer", "trousers", "shirt", "smart_shoes", "coat", "tie", "blouse", "heels", "loafers", "belt"},
		Avoids: []string{"hoodie", "joggers", "shorts", "sneakers", "cargo", "cap"}, FormalLo: 4, FormalHi: 5, Accents: 1, Patterns: false,
		Staples: []Staple{
			{"blazer", "charcoal", "Charcoal blazer", "Works with grey, navy and black trousers."},
			{"trousers", "navy", "Navy suit trousers", "Smart for every meeting."},
			{"shirt", "white", "White shirt", "The business basic."},
			{"shirt", "light blue", "Light blue shirt", "The second business basic."},
			{"smart_shoes", "black", "Black smart shoes", "Meetings and the office."},
			{"belt", "black", "Black leather belt", "Matches the smart shoes."},
		}},
	{ID: "boho", Name: "Boho", Description: "Flowing dresses, earthy colours and patterns, sandals and boots, layers of jewellery.",
		Palette: []string{"rust", "mustard", "olive", "cream", "brown", "camel", "burgundy", "beige"}, Prefers: []string{"dress", "skirt", "blouse", "sandals", "boots", "cardigan", "hat", "jewellery", "scarf"},
		Avoids: []string{"smart_shoes", "tie", "blazer"}, FormalLo: 1, FormalHi: 3, Accents: 3, Patterns: true,
		Staples: []Staple{
			{"dress", "rust", "Flowing midi dress", "The heart of a boho wardrobe."},
			{"boots", "brown", "Brown suede boots", "With dresses and skirts, spring to autumn."},
			{"cardigan", "cream", "Long chunky cardigan", "The cosy layer."},
			{"hat", "camel", "Wide-brim hat", "Finishes it, and keeps the sun off."},
		}},
	{ID: "romantic", Name: "Romantic", Description: "Soft colours and soft fabrics: dresses, blouses, skirts, delicate details.",
		Palette: []string{"blush", "pink", "lavender", "cream", "white", "light blue", "beige"}, Prefers: []string{"dress", "blouse", "skirt", "cardigan", "flats", "heels", "jewellery"},
		Avoids: []string{"cargo", "joggers", "hoodie", "sneakers"}, FormalLo: 2, FormalHi: 4, Accents: 2, Patterns: true,
		Staples: []Staple{
			{"blouse", "cream", "Cream blouse", "Soft and easy with skirts and trousers."},
			{"skirt", "blush", "Pleated midi skirt", "Moves well, dresses up and down."},
			{"cardigan", "lavender", "Soft cardigan", "Over dresses when it gets cool."},
			{"flats", "beige", "Beige ballet flats", "Comfortable and pretty."},
		}},
	{ID: "edgy", Name: "Rock / edgy", Description: "Black and dark colours, leather jackets, boots, band T-shirts, silver.",
		Palette: []string{"black", "charcoal", "grey", "burgundy", "white", "denim"}, Prefers: []string{"leather_jacket", "boots", "jeans", "tshirt", "hoodie", "jewellery"},
		Avoids: []string{"polo", "loafers", "blouse"}, FormalLo: 1, FormalHi: 3, Accents: 1, Patterns: true,
		Staples: []Staple{
			{"leather_jacket", "black", "Black leather jacket", "The piece the whole style is built around."},
			{"jeans", "black", "Black skinny or straight jeans", "With every dark top."},
			{"boots", "black", "Black boots", "Every edgy outfit ends in boots."},
			{"tshirt", "charcoal", "Washed grey or black tee", "Under the jacket."},
		}},
	{ID: "preppy", Name: "Preppy", Description: "Polo and oxford shirts, chinos, cardigans and loafers; stripes and navy.",
		Palette: []string{"navy", "white", "red", "green", "light blue", "camel", "beige", "burgundy"}, Prefers: []string{"polo", "shirt", "chinos", "cardigan", "loafers", "sweater", "blazer", "skirt", "trench"},
		Avoids: []string{"hoodie", "cargo", "joggers"}, FormalLo: 2, FormalHi: 4, Accents: 2, Patterns: true,
		Staples: []Staple{
			{"polo", "navy", "Navy polo shirt", "Smarter than a T-shirt, easy in summer."},
			{"chinos", "beige", "Stone chinos", "With navy, white and every blue."},
			{"cardigan", "navy", "Navy cardigan", "Over shirts and polos."},
			{"loafers", "brown", "Brown loafers", "Preppy shoes."},
			{"shirt", "white", "White oxford shirt", "The other preppy basic."},
		}},
	{ID: "colourful", Name: "Bold colour", Description: "Colour blocking: two or three strong colours together, worn with confidence.",
		Palette: []string{"red", "blue", "yellow", "green", "pink", "orange", "purple", "teal", "white"}, Prefers: []string{"dress", "sweater", "trousers", "blouse", "sneakers", "skirt"},
		Avoids: []string{}, FormalLo: 1, FormalHi: 4, Accents: 3, Patterns: true,
		Staples: []Staple{
			{"sweater", "red", "A red knit", "A strong colour that goes with denim, navy and camel."},
			{"trousers", "teal", "Coloured trousers", "Pair with white or a second bright."},
			{"sneakers", "white", "White trainers", "Calm shoes under bright clothes."},
			{"dress", "yellow", "A bright dress", "One piece, the whole outfit."},
		}},
	{ID: "outdoor", Name: "Outdoorsy", Description: "Practical and warm: fleeces, rain jackets, parkas, cargo trousers and boots.",
		Palette: []string{"olive", "green", "brown", "navy", "grey", "khaki", "rust", "black"}, Prefers: []string{"fleece", "raincoat", "parka", "cargo", "boots", "winter_boots", "beanie", "jeans", "longsleeve", "backpack"},
		Avoids: []string{"heels", "tie", "smart_shoes", "blouse"}, FormalLo: 1, FormalHi: 2, Accents: 2, Patterns: true,
		Staples: []Staple{
			{"raincoat", "olive", "Rain jacket", "Keeps the rain out on walks and in town."},
			{"fleece", "green", "Fleece", "The warm middle layer."},
			{"winter_boots", "brown", "Waterproof boots", "For wet and snowy days."},
			{"beanie", "grey", "Wool beanie", "Cold days."},
		}},
}

// StyleByID finds a style.
func StyleByID(id string) (Style, bool) {
	for _, s := range Styles {
		if s.ID == id {
			return s, true
		}
	}
	return Style{}, false
}

// Occasion is what an outfit is for.
type Occasion struct {
	ID        string  `json:"id"`
	Label     string  `json:"label"`
	Formality float64 `json:"formality"`
	Icon      string  `json:"icon"`
	Active    bool    `json:"active,omitempty"` // sportswear wanted
}

// Occasions to dress for.
var Occasions = []Occasion{
	{"everyday", "Everyday", 2, "sun", false},
	{"work", "Work", 3, "briefcase", false},
	{"formal_work", "Important meeting", 4, "briefcase", false},
	{"interview", "Job interview", 4, "user", false},
	{"date", "Date", 3, "heart", false},
	{"dinner", "Dinner out", 3, "utensils", false},
	{"party", "Party", 3, "sparkles", false},
	{"wedding", "Wedding", 4.6, "gift", false},
	{"formal", "Formal event", 4.5, "star", false},
	{"travel", "Travel day", 2, "plane", false},
	{"outdoors", "Outdoors", 1.3, "mountain", false},
	{"sport", "Sport or gym", 1, "run", true},
	{"home", "At home", 1.3, "home", false},
}

// OccasionByID finds an occasion (everyday when unknown).
func OccasionByID(id string) Occasion {
	for _, o := range Occasions {
		if o.ID == id {
			return o
		}
	}
	return Occasions[0]
}
