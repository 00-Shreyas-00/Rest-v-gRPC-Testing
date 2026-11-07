package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync/atomic"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	_ "google.golang.org/grpc/encoding/gzip" // registers the gzip compressor
	"google.golang.org/grpc/mem"
	"google.golang.org/protobuf/proto"
)

// Bytes on the wire (TCP payload, after compression, including HTTP and
// HTTP/2 framing) of every benchmark connection. Metrics scrapes use a
// separate, uncounted client.
var wireRx, wireTx atomic.Int64

type countingConn struct{ net.Conn }

func (c countingConn) Read(b []byte) (int, error) {
	n, err := c.Conn.Read(b)
	wireRx.Add(int64(n))
	return n, err
}

func (c countingConn) Write(b []byte) (int, error) {
	n, err := c.Conn.Write(b)
	wireTx.Add(int64(n))
	return n, err
}

func dial(ctx context.Context, network, addr string) (net.Conn, error) {
	d := net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	c, err := d.DialContext(ctx, network, addr)
	if err != nil {
		return nil, err
	}
	return countingConn{c}, nil
}

// newHTTPClient returns a client with exactly one HTTP/1.1 keep-alive
// connection. With gzip it sends Accept-Encoding: gzip and decompresses
// transparently (decompression is client CPU, as for gRPC).
func newHTTPClient(gzip bool, timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			DialContext:         dial,
			MaxConnsPerHost:     1,
			MaxIdleConnsPerHost: 1,
			IdleConnTimeout:     90 * time.Second,
			DisableCompression:  !gzip,
			ReadBufferSize:      64 << 10,
			WriteBufferSize:     64 << 10,
		},
	}
}

// newGRPCConn returns a connection (one HTTP/2 connection) with the same
// 64 MB message limit as the server. raw: responses are counted but not
// decoded (see rawCodec).
func newGRPCConn(addr string, raw bool) (*grpc.ClientConn, error) {
	callOpts := []grpc.CallOption{grpc.MaxCallRecvMsgSize(64 << 20), grpc.MaxCallSendMsgSize(64 << 20)}
	if raw {
		callOpts = append(callOpts, grpc.ForceCodecV2(rawCodec{}))
	}
	cc, err := grpc.NewClient("passthrough:///"+addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(ctx context.Context, a string) (net.Conn, error) { return dial(ctx, "tcp", a) }),
		grpc.WithDefaultCallOptions(callOpts...))
	if err != nil {
		return nil, err
	}
	cc.Connect()
	return cc, nil
}

// rawMsg is a message the client does not decode: the REST client reads
// response bodies without parsing them, and the gRPC client counts response
// messages without unmarshalling them, so client decode cost cannot limit the
// server measurement (k6 lesson from experiment 1). Requests can carry
// pre-encoded bytes. `loadgen verify` decodes everything instead.
type rawMsg struct {
	b    []byte
	n    int
	keep bool // keep a copy of the received bytes
}

type rawCodec struct{}

func (rawCodec) Name() string { return "proto" }

func (rawCodec) Marshal(v any) (mem.BufferSlice, error) {
	switch m := v.(type) {
	case *rawMsg:
		return mem.BufferSlice{mem.SliceBuffer(m.b)}, nil
	case proto.Message:
		b, err := proto.Marshal(m)
		return mem.BufferSlice{mem.SliceBuffer(b)}, err
	}
	return nil, fmt.Errorf("rawCodec: cannot marshal %T", v)
}

func (rawCodec) Unmarshal(data mem.BufferSlice, v any) error {
	switch m := v.(type) {
	case *rawMsg:
		m.n = data.Len()
		if m.keep {
			m.b = data.Materialize()
		}
		return nil
	case proto.Message:
		return proto.Unmarshal(data.Materialize(), m)
	}
	return fmt.Errorf("rawCodec: cannot unmarshal into %T", v)
}
