package grpcshim

//go:generate counterfeiter -o grpc_fake/fake_clientconn.go . ClientConn
type ClientConn interface {
	Close() error
}
