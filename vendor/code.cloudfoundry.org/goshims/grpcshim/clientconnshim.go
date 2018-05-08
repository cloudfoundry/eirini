package grpcshim

import "google.golang.org/grpc"

type ClientConnShim struct{
	ClientConn *grpc.ClientConn
}

func (c *ClientConnShim) Close() error {
	return c.ClientConn.Close()
}