package x2

import (
	"context"
	"errors"
	"io"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	grpcgzip "google.golang.org/grpc/encoding/gzip"
	"google.golang.org/grpc/status"

	"go-vs-ts-benchmark/go-server/gen/x2v1"
	"go-vs-ts-benchmark/go-server/internal/x2data"
)

// RegisterGRPC adds X2Service to s.
func init() {
	// Same gzip level as the REST servers (only used when a client compresses).
	if err := grpcgzip.SetLevel(GzipLevel); err != nil {
		panic(err)
	}
}

func RegisterGRPC(s *grpc.Server) {
	x2v1.RegisterX2ServiceServer(s, &grpcServer{hub: NewHub(nil)})
}

// ToPB converts an item (also used by the load generator).
func ToPB(it *x2data.Item) *x2v1.Item {
	return &x2v1.Item{
		Id: it.ID, Sku: it.SKU, Name: it.Name, Price: it.Price, Stock: it.Stock,
		Description: it.Description, Timestamps: it.Timestamps, Values: it.Values, Thumbnail: it.Thumbnail,
	}
}

func toPBs(items []x2data.Item) []*x2v1.Item {
	out := make([]*x2v1.Item, len(items))
	for i := range items {
		out[i] = ToPB(&items[i])
	}
	return out
}

type pbDataset struct {
	small    []*x2v1.Item
	profiles map[string][]*x2v1.Item
}

var (
	pbOnce sync.Once
	pbData *pbDataset
)

// getPB returns the data set as prebuilt protobuf messages, so per request the
// gRPC server only serializes, like the REST servers.
func getPB() *pbDataset {
	pbOnce.Do(func() {
		d := getData()
		p := &pbDataset{small: toPBs(d.small), profiles: map[string][]*x2v1.Item{}}
		for name, items := range d.profiles {
			p.profiles[name] = toPBs(items)
		}
		pbData = p
	})
	return pbData
}

type grpcServer struct {
	x2v1.UnimplementedX2ServiceServer
	hub *Hub
}

func invalid(err error) error { return status.Error(codes.InvalidArgument, err.Error()) }

func (s *grpcServer) GetPayload(_ context.Context, req *x2v1.PayloadRequest) (*x2v1.ItemList, error) {
	if err := x2data.CheckProfile(req.GetProfile()); err != nil {
		return nil, invalid(err)
	}
	if err := x2data.CheckRange("count", int64(req.GetCount()), 1, x2data.PoolSize); err != nil {
		return nil, invalid(err)
	}
	return &x2v1.ItemList{Items: getPB().profiles[req.GetProfile()][:req.GetCount()]}, nil
}

func (s *grpcServer) ExportUnary(_ context.Context, req *x2v1.ExportRequest) (*x2v1.ItemList, error) {
	if err := x2data.CheckRange("count", int64(req.GetCount()), 1, x2data.ExportSize); err != nil {
		return nil, invalid(err)
	}
	return &x2v1.ItemList{Items: getPB().small[:req.GetCount()]}, nil
}

func (s *grpcServer) ExportStream(req *x2v1.ExportRequest, stream grpc.ServerStreamingServer[x2v1.Item]) error {
	if err := x2data.CheckRange("count", int64(req.GetCount()), 1, x2data.ExportSize); err != nil {
		return invalid(err)
	}
	for _, it := range getPB().small[:req.GetCount()] {
		if err := stream.Send(it); err != nil {
			return err
		}
	}
	return nil
}

func (s *grpcServer) ImportUnary(_ context.Context, req *x2v1.ItemList) (*x2v1.ImportSummary, error) {
	var sum x2data.ImportSummary
	for _, it := range req.GetItems() {
		sum.Add(it.GetId(), it.GetStock())
	}
	return &x2v1.ImportSummary{Count: sum.Count, IdSum: sum.IDSum, StockSum: sum.StockSum}, nil
}

func (s *grpcServer) ImportStream(stream grpc.ClientStreamingServer[x2v1.Item, x2v1.ImportSummary]) error {
	var sum x2data.ImportSummary
	for {
		it, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return stream.SendAndClose(&x2v1.ImportSummary{Count: sum.Count, IdSum: sum.IDSum, StockSum: sum.StockSum})
		}
		if err != nil {
			return err
		}
		sum.Add(it.GetId(), it.GetStock())
	}
}

func (s *grpcServer) Subscribe(req *x2v1.SubscribeRequest, stream grpc.ServerStreamingServer[x2v1.PriceEvent]) error {
	if err := x2data.CheckRange("rate", int64(req.GetRate()), 1, x2data.MaxRate); err != nil {
		return invalid(err)
	}
	events, cancel, err := s.hub.Subscribe(int(req.GetRate()))
	if err != nil {
		return status.Error(codes.FailedPrecondition, err.Error())
	}
	defer cancel()
	ctx := stream.Context()
	for {
		select {
		case <-ctx.Done():
			return nil
		case ev := <-events:
			if err := stream.Send(&x2v1.PriceEvent{Seq: ev.Seq, ItemId: ev.ItemID, Price: ev.Price, SentAtUs: ev.SentAtUs}); err != nil {
				return err
			}
		}
	}
}

func quoteFor(id int64) (*x2v1.Quote, error) {
	if err := x2data.CheckRange("id", id, 1, x2data.ExportSize); err != nil {
		return nil, invalid(err)
	}
	it := getPB().small[id-1]
	return &x2v1.Quote{Id: it.Id, Price: it.Price, Stock: it.Stock}, nil
}

func (s *grpcServer) GetQuote(_ context.Context, req *x2v1.QuoteRequest) (*x2v1.Quote, error) {
	return quoteFor(req.GetId())
}

func (s *grpcServer) QuoteStream(stream grpc.BidiStreamingServer[x2v1.QuoteRequest, x2v1.Quote]) error {
	for {
		req, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		q, err := quoteFor(req.GetId())
		if err != nil {
			return err
		}
		if err := stream.Send(q); err != nil {
			return err
		}
	}
}
