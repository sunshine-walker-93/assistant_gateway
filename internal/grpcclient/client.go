package grpcclient

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	reflectionv1alpha "google.golang.org/grpc/reflection/grpc_reflection_v1alpha"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
)

// ClientManager maintains gRPC connections to downstream services.
// For MVP it does simple address-based pooling without service discovery.
type ClientManager struct {
	logger *zap.Logger

	mu    sync.Mutex
	conns map[string]*grpc.ClientConn // key: backend address

	// Reflection client cache
	reflectionClients map[string]reflectionv1alpha.ServerReflectionClient

	// Type name cache: fullMethod -> (requestType, responseType)
	typeCache map[string]typeInfo

	// FileDescriptorSet cache: addr -> Files (from protodesc)
	filesCache map[string]*protoregistry.Files
}

// typeInfo stores request and response type names for a method
type typeInfo struct {
	requestType  string
	responseType string
}

// NewClientManager creates a new ClientManager.
func NewClientManager(logger *zap.Logger) *ClientManager {
	return &ClientManager{
		logger:            logger,
		conns:             make(map[string]*grpc.ClientConn),
		reflectionClients: make(map[string]reflectionv1alpha.ServerReflectionClient),
		typeCache:         make(map[string]typeInfo),
		filesCache:        make(map[string]*protoregistry.Files),
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

// parseMethodName parses a gRPC method name like "/user.v1.UserService/Login"
// and returns the service name (e.g., "user.v1") and method name (e.g., "Login").
func parseMethodName(fullMethod string) (serviceName, methodName string) {
	// Remove leading slash
	if len(fullMethod) > 0 && fullMethod[0] == '/' {
		fullMethod = fullMethod[1:]
	}

	// Find last dot before method name
	// Format: user.v1.UserService.Login or user.v1.UserService/Login
	lastDot := -1
	lastSlash := -1
	for i := len(fullMethod) - 1; i >= 0; i-- {
		if fullMethod[i] == '.' && lastDot == -1 {
			lastDot = i
		}
		if fullMethod[i] == '/' && lastSlash == -1 {
			lastSlash = i
		}
	}

	// Extract method name
	if lastSlash != -1 {
		methodName = fullMethod[lastSlash+1:]
		fullMethod = fullMethod[:lastSlash]
	} else if lastDot != -1 {
		methodName = fullMethod[lastDot+1:]
		fullMethod = fullMethod[:lastDot]
	} else {
		return "", ""
	}

	// Extract service name (everything before the last component)
	// user.v1.UserService -> user.v1
	lastDot = -1
	for i := len(fullMethod) - 1; i >= 0; i-- {
		if fullMethod[i] == '.' {
			lastDot = i
			break
		}
	}

	if lastDot != -1 {
		serviceName = fullMethod[:lastDot]
	} else {
		serviceName = fullMethod
	}

	return serviceName, methodName
}

// getReflectionClient returns a cached reflection client for the given address.
func (m *ClientManager) getReflectionClient(ctx context.Context, addr string) (reflectionv1alpha.ServerReflectionClient, error) {
	m.mu.Lock()
	client, ok := m.reflectionClients[addr]
	m.mu.Unlock()
	if ok {
		return client, nil
	}

	conn, err := m.GetConn(ctx, addr)
	if err != nil {
		return nil, err
	}

	client = reflectionv1alpha.NewServerReflectionClient(conn)

	m.mu.Lock()
	m.reflectionClients[addr] = client
	m.mu.Unlock()

	return client, nil
}

// getTypeNamesFromReflection uses gRPC reflection to get request/response type names for a method.
func (m *ClientManager) getTypeNamesFromReflection(ctx context.Context, addr, fullMethod string) (requestType, responseType string, err error) {
	client, err := m.getReflectionClient(ctx, addr)
	if err != nil {
		return "", "", fmt.Errorf("failed to get reflection client: %w", err)
	}

	serviceName, methodName := parseMethodName(fullMethod)
	if serviceName == "" || methodName == "" {
		return "", "", fmt.Errorf("failed to parse method name: %s", fullMethod)
	}

	// Query for file containing the service
	stream, err := client.ServerReflectionInfo(ctx)
	if err != nil {
		return "", "", fmt.Errorf("failed to create reflection stream: %w", err)
	}
	defer stream.CloseSend()

	// Request file by symbol (service name)
	serviceSymbol := serviceName + "." + extractServiceTypeName(fullMethod)
	if err := stream.Send(&reflectionv1alpha.ServerReflectionRequest{
		MessageRequest: &reflectionv1alpha.ServerReflectionRequest_FileContainingSymbol{
			FileContainingSymbol: serviceSymbol,
		},
	}); err != nil {
		return "", "", fmt.Errorf("failed to send reflection request: %w", err)
	}

	resp, err := stream.Recv()
	if err != nil {
		return "", "", fmt.Errorf("failed to receive reflection response: %w", err)
	}

	if resp.GetErrorResponse() != nil {
		return "", "", fmt.Errorf("reflection error: %d - %s", resp.GetErrorResponse().GetErrorCode(), resp.GetErrorResponse().GetErrorMessage())
	}

	fileResp := resp.GetFileDescriptorResponse()
	if fileResp == nil {
		return "", "", fmt.Errorf("unexpected reflection response type")
	}

	// Parse FileDescriptorSet to find method and extract types
	// FileDescriptorProto is [][]byte, need to parse each one
	var fileDescs []*descriptorpb.FileDescriptorProto
	for _, fdBytes := range fileResp.FileDescriptorProto {
		fd := &descriptorpb.FileDescriptorProto{}
		if err := proto.Unmarshal(fdBytes, fd); err != nil {
			continue // Skip invalid descriptors
		}
		fileDescs = append(fileDescs, fd)
	}

	requestType, responseType, err = extractTypesFromFileDescriptorSet(fileDescs, serviceName, methodName)
	if err != nil {
		return "", "", fmt.Errorf("failed to extract types: %w", err)
	}

	return requestType, responseType, nil
}

// getFilesFromReflection gets Files from gRPC reflection and caches them.
func (m *ClientManager) getFilesFromReflection(ctx context.Context, addr string) (*protoregistry.Files, error) {
	// Check cache first
	m.mu.Lock()
	files, ok := m.filesCache[addr]
	m.mu.Unlock()
	if ok {
		return files, nil
	}

	client, err := m.getReflectionClient(ctx, addr)
	if err != nil {
		return nil, fmt.Errorf("failed to get reflection client: %w", err)
	}

	// Query for all services to get FileDescriptorSet
	stream, err := client.ServerReflectionInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create reflection stream: %w", err)
	}
	defer stream.CloseSend()

	// Request list of services
	if err := stream.Send(&reflectionv1alpha.ServerReflectionRequest{
		MessageRequest: &reflectionv1alpha.ServerReflectionRequest_ListServices{
			ListServices: "",
		},
	}); err != nil {
		return nil, fmt.Errorf("failed to send list services request: %w", err)
	}

	resp, err := stream.Recv()
	if err != nil {
		return nil, fmt.Errorf("failed to receive list services response: %w", err)
	}

	if resp.GetErrorResponse() != nil {
		return nil, fmt.Errorf("reflection error: %d - %s", resp.GetErrorResponse().GetErrorCode(), resp.GetErrorResponse().GetErrorMessage())
	}

	listResp := resp.GetListServicesResponse()
	if listResp == nil {
		return nil, fmt.Errorf("unexpected reflection response type")
	}

	// Collect all FileDescriptorSets by querying each service
	var allFileDescs []*descriptorpb.FileDescriptorProto
	seenFiles := make(map[string]bool)

	for _, service := range listResp.GetService() {
		serviceName := service.GetName()
		// Request file containing this service
		if err := stream.Send(&reflectionv1alpha.ServerReflectionRequest{
			MessageRequest: &reflectionv1alpha.ServerReflectionRequest_FileContainingSymbol{
				FileContainingSymbol: serviceName,
			},
		}); err != nil {
			m.logger.Warn("failed to send file request for service", zap.String("service", serviceName), zap.Error(err))
			continue
		}

		fileResp, err := stream.Recv()
		if err != nil {
			m.logger.Warn("failed to receive file response", zap.String("service", serviceName), zap.Error(err))
			continue
		}

		if fileResp.GetErrorResponse() != nil {
			m.logger.Warn("reflection error for service", zap.String("service", serviceName), zap.Error(err))
			continue
		}

		fileDescResp := fileResp.GetFileDescriptorResponse()
		if fileDescResp == nil {
			continue
		}

		// Parse FileDescriptorProto
		for _, fdBytes := range fileDescResp.FileDescriptorProto {
			fd := &descriptorpb.FileDescriptorProto{}
			if err := proto.Unmarshal(fdBytes, fd); err != nil {
				continue
			}

			// Avoid duplicates
			if seenFiles[fd.GetName()] {
				continue
			}
			seenFiles[fd.GetName()] = true
			allFileDescs = append(allFileDescs, fd)
		}
	}

	// Create FileDescriptorSet
	fds := &descriptorpb.FileDescriptorSet{
		File: allFileDescs,
	}

	// Create Files from FileDescriptorSet
	files, err = protodesc.NewFiles(fds)
	if err != nil {
		return nil, fmt.Errorf("failed to create Files from FileDescriptorSet: %w", err)
	}

	// Cache the result
	m.mu.Lock()
	m.filesCache[addr] = files
	m.mu.Unlock()

	return files, nil
}

// getMessageTypeFromFiles gets a MessageType from Files by type name.
// This uses dynamicpb.NewMessageType to create a dynamic message type from the descriptor.
func (m *ClientManager) getMessageTypeFromFiles(files *protoregistry.Files, typeName string) (protoreflect.MessageType, error) {
	// Remove leading dot if present
	if len(typeName) > 0 && typeName[0] == '.' {
		typeName = typeName[1:]
	}

	// Find the message descriptor in Files
	desc, err := files.FindDescriptorByName(protoreflect.FullName(typeName))
	if err != nil {
		return nil, fmt.Errorf("message type not found: %s: %w", typeName, err)
	}

	msgDesc, ok := desc.(protoreflect.MessageDescriptor)
	if !ok {
		return nil, fmt.Errorf("descriptor is not a message: %s", typeName)
	}

	// Create a dynamic message type using dynamicpb
	return dynamicpb.NewMessageType(msgDesc), nil
}

// extractServiceTypeName extracts the service type name from fullMethod.
// e.g., "/user.v1.UserService/Login" -> "UserService"
func extractServiceTypeName(fullMethod string) string {
	// Remove leading slash
	if len(fullMethod) > 0 && fullMethod[0] == '/' {
		fullMethod = fullMethod[1:]
	}

	// Find the service name part
	// Format: user.v1.UserService/Login
	lastSlash := -1
	for i := len(fullMethod) - 1; i >= 0; i-- {
		if fullMethod[i] == '/' {
			lastSlash = i
			break
		}
	}

	if lastSlash == -1 {
		return ""
	}

	servicePart := fullMethod[:lastSlash]
	// Find last dot in service part
	lastDot := -1
	for i := len(servicePart) - 1; i >= 0; i-- {
		if servicePart[i] == '.' {
			lastDot = i
			break
		}
	}

	if lastDot == -1 {
		return servicePart
	}

	return servicePart[lastDot+1:]
}

// extractTypesFromFileDescriptorSet extracts request/response type names from FileDescriptorSet.
func extractTypesFromFileDescriptorSet(fds []*descriptorpb.FileDescriptorProto, serviceName, methodName string) (requestType, responseType string, err error) {
	// Find the service in the file descriptors
	for _, fd := range fds {
		pkg := fd.GetPackage()
		for _, svc := range fd.GetService() {
			// Construct full service name
			fullServiceName := pkg
			if fullServiceName != "" {
				fullServiceName += "."
			}
			fullServiceName += svc.GetName()

			// Match service name (e.g., "user.v1.UserService")
			// serviceName from parseMethodName is "user.v1", svc.GetName() is "UserService"
			expectedServiceName := serviceName + "." + svc.GetName()
			if fullServiceName == expectedServiceName {
				// Find the method
				for _, method := range svc.GetMethod() {
					if method.GetName() == methodName {
						// Extract request and response type names
						// These are already fully qualified (e.g., ".user.v1.LoginRequest" or "user.v1.LoginRequest")
						requestType = method.GetInputType()
						responseType = method.GetOutputType()
						// Remove leading dot if present
						if len(requestType) > 0 && requestType[0] == '.' {
							requestType = requestType[1:]
						}
						if len(responseType) > 0 && responseType[0] == '.' {
							responseType = responseType[1:]
						}
						return requestType, responseType, nil
					}
				}
			}
		}
	}

	return "", "", fmt.Errorf("method %s not found in service %s", methodName, serviceName)
}

// getTypeNames gets request/response type names for a method, using cache or reflection.
func (m *ClientManager) getTypeNames(ctx context.Context, addr, fullMethod string) (requestType, responseType string, err error) {
	// Check cache first
	m.mu.Lock()
	info, ok := m.typeCache[fullMethod]
	m.mu.Unlock()
	if ok {
		return info.requestType, info.responseType, nil
	}

	// Use reflection to get type names (no inference needed, pure reflection approach)
	requestType, responseType, err = m.getTypeNamesFromReflection(ctx, addr, fullMethod)
	if err != nil {
		return "", "", err
	}

	// Cache the result
	m.mu.Lock()
	m.typeCache[fullMethod] = typeInfo{
		requestType:  requestType,
		responseType: responseType,
	}
	m.mu.Unlock()

	return requestType, responseType, nil
}

// InvokeJSON uses gRPC reflection to dynamically get request/response types,
// then converts JSON to proto messages and back. Falls back to structpb.Struct
// if reflection fails (for backward compatibility).
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

	// Get Files from reflection (cached)
	files, err := m.getFilesFromReflection(ctx, addr)
	if err != nil {
		return nil, fmt.Errorf("failed to get Files from reflection: %w", err)
	}

	// Try to get type names using reflection or inference
	requestType, responseType, err := m.getTypeNames(ctx, addr, fullMethod)
	if err != nil {
		return nil, fmt.Errorf("failed to get type names: %w", err)
	}

	// Get message types from Files
	reqMsgType, err := m.getMessageTypeFromFiles(files, requestType)
	if err != nil {
		return nil, fmt.Errorf("failed to get request message type: %w", err)
	}

	respMsgType, err := m.getMessageTypeFromFiles(files, responseType)
	if err != nil {
		return nil, fmt.Errorf("failed to get response message type: %w", err)
	}

	// Create proto message instances
	reqMsg := reqMsgType.New().Interface().(proto.Message)
	respMsg := respMsgType.New().Interface().(proto.Message)

	// Convert JSON to proto request
	if len(reqJSON) > 0 {
		if err := protojson.Unmarshal(reqJSON, reqMsg); err != nil {
			return nil, fmt.Errorf("failed to unmarshal JSON to proto: %w", err)
		}
	}

	// Invoke gRPC method
	if err := conn.Invoke(ctx, fullMethod, reqMsg, respMsg); err != nil {
		return nil, err
	}

	// Convert proto response to JSON
	marshaler := protojson.MarshalOptions{
		EmitUnpopulated: true,
	}
	respJSON, err := marshaler.Marshal(respMsg)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal proto to JSON: %w", err)
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
