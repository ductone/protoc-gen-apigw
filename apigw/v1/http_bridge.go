package v1

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
)

func MetadataForRequest(req *http.Request, methodFulLName string) metadata.MD {
	// https://github.com/grpc/grpc/blob/master/doc/PROTOCOL-HTTP2.md
	rv := metadata.MD{}
	for k, v := range req.Header {
		rv.Set(k, v...)
	}

	// emulate grpc-go's behavior of setting these headers.
	// TODO(pquerna): should we do this or preserve the original headers?
	rv.Set("content-type ", "application/grpc+proto")
	rv.Set(":method", "POST")
	rv.Set(":path", methodFulLName)
	rv.Set(":authority", req.Host)
	rv.Set(":scheme", "https")

	return rv
}

func PeerForRequest(req *http.Request) *peer.Peer {
	// TODO(pquerna): grpc-server uses a raw conn address here.
	return &peer.Peer{
		Addr: strAddr(req.RemoteAddr),
	}
}

func TimeoutForRequest(req *http.Request) (time.Duration, bool) {
	// TODO(pquerna): supporty grpc-timeout
	// if hdr := req.Header.Get("grpc-timeout"); hdr != "" {
	// 	return
	// }
	return 0, false
}

type RouteToken struct {
	IsParam   bool
	ParamName string
	Value     string
}

// Parses a HTP Route path into a list of RouteTokens.
// For example a route can be:
//
//	/v1/thng/{app_id}
//	/v1/users/{user_id}/devices/{device_id}
//
// This is NOT a fully compliant port of the gRPC HTTP API spec, but
// its "good enough" for now.
func ParseRoute(path string) ([]RouteToken, error) {
	if !strings.HasPrefix(path, "/") {
		return nil, fmt.Errorf("apigw_v1: invalid route: must start with '/'")
	}

	rv := make([]RouteToken, 0)
	for _, token := range strings.Split(path, "/") {
		if token == "" {
			continue
		}
		if strings.HasPrefix(token, "{") && strings.HasSuffix(token, "}") {
			rv = append(rv, RouteToken{
				IsParam:   true,
				ParamName: token[1 : len(token)-1],
			})
		} else {
			rv = append(rv, RouteToken{
				IsParam: false,
				Value:   token,
			})
		}
	}
	return rv, nil
}
