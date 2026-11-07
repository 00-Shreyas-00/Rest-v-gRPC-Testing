// Command loadgen is the experiment-2 load generator (large payloads and
// streaming). One client implementation drives every server, so protocol
// differences are not mixed with load-tool differences.
//
//	loadgen run    [flags]   one measured run; prints one JSON line
//	loadgen verify [flags]   equivalence check of every x2 endpoint on every server
//	loadgen codec  [flags]   JSON vs Protobuf encode/decode micro-benchmark (no network)
//
// It runs inside the loadgen container on the compose network (see
// docker-compose.yml, profile x2) and is driven by scripts/run-x2.sh.
// Methodology: docs/experiment-2-design.md.
package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: loadgen run|verify|codec [flags]  (-h for flags)")
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "run":
		err = cmdRun(os.Args[2:])
	case "verify":
		err = cmdVerify(os.Args[2:])
	case "codec":
		err = cmdCodec(os.Args[2:])
	default:
		err = fmt.Errorf("unknown command %q", os.Args[1])
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "loadgen:", err)
		os.Exit(1)
	}
}

// Server addresses on the compose network.
func targetAddr(target string) (addr string, grpc bool, err error) {
	switch target {
	case "go-rest", "ts-rest":
		return target + ":8080", false, nil
	case "go-grpc":
		return target + ":9090", true, nil
	}
	return "", false, fmt.Errorf("unknown target %q (go-rest, ts-rest, go-grpc)", target)
}
