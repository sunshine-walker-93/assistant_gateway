package grpcclient

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/known/structpb"
)

// ClientManager maintains gRPC connections to downstream services.
// For MVP it does simple address-based pooling without service discovery.
type ClientManager struct {
	logger *zap.Logger

	mu    sync.Mutex
	conns map[string]*grpc.ClientConn // key: backend address
}

// NewClientManager creates a new ClientManager.
func NewClientManager(logger *zap.Logger) *ClientManager {
	return &ClientManager{
		logger: logger,
		conns:  make(map[string]*grpc.ClientConn),
	}
}

// GetConn returns a cached connection for the given address, dialing it if needed.
func (m *ClientManager) GetConn(ctx context.Context, addr string) (*grpc.ClientConn, error) {
	m.mu.Lock()
	conn, ok := m.conns[addr]
	m.mu.Unlock()
	if ok {
		return conn, nil
	}

	// Dial with a bounded timeout.
	dialCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	newConn, err := grpc.DialContext(
		dialCtx,
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, err
	}

	m.mu.Lock()
	m.conns[addr] = newConn
	m.mu.Unlock()
	return newConn, nil
}

// InvokeJSON assumes both request and response use google.protobuf.Struct as payload.
// It converts arbitrary JSON into Struct and back, which is a pragmatic MVP approach
// before wiring full proto descriptors.
func (m *ClientManager) InvokeJSON(
	ctx context.Context,
	addr string,
	fullMethod string,
	reqJSON json.RawMessage,
) (json.RawMessage, error) {
	conn, err := m.GetConn(ctx, addr)
	if err != nil {
		return nil, err
	}

	// Convert JSON to Struct.
	var raw map[string]interface{}
	if len(reqJSON) > 0 {
		if err := json.Unmarshal(reqJSON, &raw); err != nil {
			return nil, err
		}
	}
	reqStruct, err := structpb.NewStruct(raw)
	if err != nil {
		return nil, err
	}

	// Response Struct.
	respStruct := &structpb.Struct{}
	if err := conn.Invoke(ctx, fullMethod, reqStruct, respStruct); err != nil {
		return nil, err
	}

	// Convert back to JSON.
	respJSON, err := json.Marshal(respStruct.AsMap())
	if err != nil {
		return nil, err
	}
	return respJSON, nil
}

// InvokeProto performs a strongly-typed gRPC invocation using JSON as the wire
// format at the HTTP boundary. It looks up the request/response proto message
// types by their fully-qualified names (e.g. "user.v1.LoginRequest") and uses
// protojson to convert between JSON and proto messages.
func (m *ClientManager) InvokeProto(
	ctx context.Context,
	addr string,
	fullMethod string,
	reqJSON json.RawMessage,
	requestType string,
	responseType string,
) (json.RawMessage, error) {
	conn, err := m.GetConn(ctx, addr)
	if err != nil {
		return nil, err
	}

	// Resolve request/response types from the global registry. As long as the
	// corresponding generated pb packages are imported, their types will be
	// registered here.
	reqMsgType, err := protoregistry.GlobalTypes.FindMessageByName(protoreflect.FullName(requestType))
	if err != nil {
		return nil, err
	}
	respMsgType, err := protoregistry.GlobalTypes.FindMessageByName(protoreflect.FullName(responseType))
	if err != nil {
		return nil, err
	}

	reqMsg := reqMsgType.New().Interface().(proto.Message)
	respMsg := respMsgType.New().Interface().(proto.Message)

	// Populate request message from JSON if body is non-empty.
	if len(reqJSON) > 0 {
		if err := protojson.Unmarshal(reqJSON, reqMsg); err != nil {
			return nil, err
		}
	}

	if err := conn.Invoke(ctx, fullMethod, reqMsg, respMsg); err != nil {
		return nil, err
	}

	marshaler := protojson.MarshalOptions{
		EmitUnpopulated: true,
	}
	out, err := marshaler.Marshal(respMsg)
	if err != nil {
		return nil, err
	}
	return out, nil
}


