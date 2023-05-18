package v1

import (
	"net/http"
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
