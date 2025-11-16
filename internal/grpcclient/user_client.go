package grpcclient

import (
	"context"

	userpb "github.com/sunshine-walker-93/assistant_account/pkg/pb/user/v1"
)

// UserServiceClient is a thin wrapper around the generated gRPC client
// that reuses the shared ClientManager connection pool.
type UserServiceClient struct {
	mgr  *ClientManager
	addr string
}

// NewUserServiceClient creates a new UserServiceClient targeting the given backend address.
func NewUserServiceClient(mgr *ClientManager, addr string) *UserServiceClient {
	return &UserServiceClient{
		mgr:  mgr,
		addr: addr,
	}
}

// Login calls user.v1.UserService.Login with a strongly-typed request/response.
func (c *UserServiceClient) Login(ctx context.Context, req *userpb.LoginRequest) (*userpb.LoginResponse, error) {
	conn, err := c.mgr.GetConn(ctx, c.addr)
	if err != nil {
		return nil, err
	}
	client := userpb.NewUserServiceClient(conn)
	return client.Login(ctx, req)
}


