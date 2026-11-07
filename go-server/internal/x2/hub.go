package x2

import (
	"errors"
	"sync"
	"time"

	"go-vs-ts-benchmark/go-server/internal/x2data"
)

// subscriberBuffer is the per-subscriber queue. When it is full (a slow
// subscriber), events are dropped for that subscriber; the client sees a gap in
// seq. The TypeScript server drops at a comparable backlog.
const subscriberBuffer = 256

// Event is a feed event plus its pre-encoded SSE frame. The hub encodes the
// JSON once per event and REST subscribers write the same bytes; gRPC
// subscribers Send the struct (grpc-go marshals per stream).
type Event struct {
	x2data.PriceEvent
	SSE []byte
}

// Hub broadcasts price events at a fixed rate to every subscriber. It runs
// only while there are subscribers; all subscribers must ask for the same rate.
type Hub struct {
	mu      sync.Mutex
	rate    int
	subs    map[chan *Event]struct{}
	stop    chan struct{}
	seq     int64
	encode  func(x2data.PriceEvent) []byte
	Dropped uint64
}

// NewHub returns a hub. encode builds the SSE frame (nil: no frame).
func NewHub(encode func(x2data.PriceEvent) []byte) *Hub {
	return &Hub{subs: map[chan *Event]struct{}{}, encode: encode}
}

var errRate = errors.New("a feed with a different rate is running")

// Subscribe registers a subscriber; call cancel when it disconnects.
func (h *Hub) Subscribe(rate int) (<-chan *Event, func(), error) {
	small := getData().small
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.subs) > 0 && rate != h.rate {
		return nil, nil, errRate
	}
	if len(h.subs) == 0 {
		h.rate = rate
		h.stop = make(chan struct{})
		go h.run(rate, h.stop, small)
	}
	ch := make(chan *Event, subscriberBuffer)
	h.subs[ch] = struct{}{}
	return ch, func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		delete(h.subs, ch)
		if len(h.subs) == 0 {
			close(h.stop)
		}
	}, nil
}

func (h *Hub) run(rate int, stop chan struct{}, small []x2data.Item) {
	t := time.NewTicker(time.Second / time.Duration(rate))
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case <-t.C:
		}
		h.mu.Lock()
		h.seq++
		pe := x2data.EventFor(h.seq, small)
		pe.SentAtUs = nowUs()
		ev := &Event{PriceEvent: pe}
		if h.encode != nil {
			ev.SSE = h.encode(pe)
		}
		for ch := range h.subs {
			select {
			case ch <- ev:
			default:
				h.Dropped++
			}
		}
		h.mu.Unlock()
	}
}
