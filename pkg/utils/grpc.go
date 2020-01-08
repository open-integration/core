package utils

import "google.golang.org/grpc"

type (
	// GRPC expose abilities on grpc connection
	GRPC struct{}
)

// Dial returns a connection grpc server
func (_g GRPC) Dial(target string, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	return grpc.Dial(target, opts...)
}
