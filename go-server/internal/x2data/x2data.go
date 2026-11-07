// Package x2data generates the deterministic data set of experiment 2
// (large payloads and streaming). The TypeScript server has a line-by-line
// port in typescript-server/src/x2/data.ts; both must produce identical items,
// which `loadgen verify` checks against every server.
//
// Everything derives from a 32-bit PRNG (mulberry32) seeded per item, using
// only operations that give identical results in Go (uint32 arithmetic,
// IEEE-754 doubles) and JavaScript (Math.imul, >>> 0, Number).
package x2data

import (
	"fmt"
	"math"
	"strings"
)

// Limits shared by every server.
const (
	PoolSize   = 4000   // payload profiles: items 1..PoolSize
	ExportSize = 100000 // small items: export, import, quotes, feed
	MaxRate    = 1000   // feed events per second

	DescriptionLen = 1000 // text profile
	SeriesLen      = 64   // numeric profile: timestamps and values
	ThumbnailLen   = 1000 // binary profile

	seriesStartMs = 1767225600000 // 2026-01-01T00:00:00Z
)

// Profiles.
const (
	Small   = "small"
	Text    = "text"
	Numeric = "numeric"
	Binary  = "binary"
)

// Profiles lists the payload profiles (GetPayload / GET /x2/payload).
var Profiles = []string{Text, Numeric, Binary}

var salts = map[string]uint32{Small: 1, Text: 2, Numeric: 3, Binary: 4}

// Item is one catalog item. JSON field order and omitempty match the
// TypeScript objects exactly, so REST bodies are byte-identical.
type Item struct {
	ID          int64     `json:"id"`
	SKU         string    `json:"sku"`
	Name        string    `json:"name"`
	Price       float64   `json:"price"`
	Stock       int32     `json:"stock"`
	Description string    `json:"description,omitempty"`
	Timestamps  []int64   `json:"timestamps,omitempty"`
	Values      []float64 `json:"values,omitempty"`
	Thumbnail   []byte    `json:"thumbnail,omitempty"`
}

// ItemList is the REST body of payload, export (json) and import (json).
type ItemList struct {
	Items []Item `json:"items"`
}

// PriceEvent is one feed event (SSE data line).
type PriceEvent struct {
	Seq      int64   `json:"seq"`
	ItemID   int64   `json:"item_id"`
	Price    float64 `json:"price"`
	SentAtUs int64   `json:"sent_at_us"`
}

// Quote is the GET /x2/quote/{id} body.
type Quote struct {
	ID    int64   `json:"id"`
	Price float64 `json:"price"`
	Stock int32   `json:"stock"`
}

// ImportSummary is the POST /x2/import response.
type ImportSummary struct {
	Count    int64 `json:"count"`
	IDSum    int64 `json:"id_sum"`
	StockSum int64 `json:"stock_sum"`
}

// Add accounts one imported item.
func (s *ImportSummary) Add(id int64, stock int32) {
	s.Count++
	s.IDSum += id
	s.StockSum += int64(stock)
}

// syllables build the description pseudo-words (1-3 syllables each): a large
// vocabulary, so the text compresses about as well as real product copy
// (gzip ~2.5-3x) rather than 6x with a tiny word list.
var syllables = []string{
	"ka", "lo", "mi", "ne", "ru", "sa", "to", "vi", "ber", "con", "dex", "fal",
	"gen", "hor", "lin", "mon", "por", "quel", "ras", "sten", "tur", "val", "wex", "zor",
}

func (r *rng) word() string {
	n := 1 + r.next()%3
	w := ""
	for range n {
		w += syllables[r.next()%24]
	}
	return w
}

var words = []string{
	"alpha", "basic", "cargo", "delta", "eagle", "fable", "giant", "harbor",
	"ivory", "jolly", "karma", "lunar", "maple", "noble", "ocean", "prism",
	"quartz", "rapid", "solar", "tiger", "ultra", "vivid", "wander", "xenon",
	"yield", "zephyr", "amber", "breeze", "cobalt", "dune", "ember", "fjord",
}

// rng is mulberry32.
type rng struct{ a uint32 }

func newRNG(id int64, salt uint32) *rng {
	return &rng{a: uint32(id)*0x9E3779B1 + salt}
}

func (r *rng) next() uint32 {
	r.a += 0x6D2B79F5
	t := r.a
	t = (t ^ t>>15) * (1 | t)
	t = (t + (t^t>>7)*(61|t)) ^ t
	return t ^ t>>14
}

// float returns a double in [0, 1) with 53 random bits.
func (r *rng) float() float64 {
	a := uint64(r.next() >> 5)
	b := uint64(r.next() >> 6)
	return float64(a*67108864+b) / 9007199254740992
}

// Generate builds the item with the given id and profile.
func Generate(id int64, profile string) Item {
	salt, ok := salts[profile]
	if !ok {
		panic("x2data: unknown profile " + profile)
	}
	base := newRNG(id, salts[Small])
	it := Item{
		ID:    id,
		SKU:   fmt.Sprintf("SKU-%07d", id),
		Name:  words[base.next()%32] + " " + words[base.next()%32] + " " + words[base.next()%32],
		Price: math.Round(base.float()*100000) / 100,
		Stock: int32(base.next() % 1000),
	}
	r := newRNG(id, salt)
	switch profile {
	case Text:
		var b strings.Builder
		for b.Len() < DescriptionLen {
			if b.Len() > 0 {
				b.WriteByte(' ')
			}
			b.WriteString(r.word())
		}
		it.Description = b.String()[:DescriptionLen]
	case Numeric:
		it.Timestamps = make([]int64, SeriesLen)
		it.Values = make([]float64, SeriesLen)
		for i := range SeriesLen {
			it.Timestamps[i] = seriesStartMs + int64(i)*60000 + int64(r.next()%1000)
			it.Values[i] = r.float() * 1000
		}
	case Binary:
		it.Thumbnail = make([]byte, ThumbnailLen)
		for i := 0; i < ThumbnailLen; i += 4 {
			v := r.next()
			it.Thumbnail[i], it.Thumbnail[i+1], it.Thumbnail[i+2], it.Thumbnail[i+3] = byte(v), byte(v>>8), byte(v>>16), byte(v>>24)
		}
	}
	return it
}

// Items returns items 1..n of a profile.
func Items(profile string, n int) []Item {
	out := make([]Item, n)
	for i := range out {
		out[i] = Generate(int64(i+1), profile)
	}
	return out
}

// EventFor returns feed event seq (without the timestamp). small must hold
// the small items 1..ExportSize.
func EventFor(seq int64, small []Item) PriceEvent {
	id := seq*7919%ExportSize + 1
	return PriceEvent{Seq: seq, ItemID: id, Price: small[id-1].Price}
}

// Validation errors, identical text in every server.
func CheckProfile(p string) error {
	for _, q := range Profiles {
		if p == q {
			return nil
		}
	}
	return fmt.Errorf("profile must be one of text, numeric, binary")
}

func CheckRange(name string, v int64, lo, hi int64) error {
	if v < lo || v > hi {
		return fmt.Errorf("%s must be between %d and %d", name, lo, hi)
	}
	return nil
}
