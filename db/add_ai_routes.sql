-- Add AI service backend and routes to Gateway configuration
-- Execute this script to add AI service integration

USE assistant_gateway_db;

-- Insert AI service backend
INSERT INTO backends (name, addr, description, enabled) VALUES
('ai', 'assistant-ai-app:50052', 'AI Service with LangChain/LangGraph agents', 1)
ON DUPLICATE KEY UPDATE 
    addr = VALUES(addr),
    description = VALUES(description),
    enabled = VALUES(enabled);

-- Insert AI service routes
INSERT INTO routes (http_method, http_pattern, backend_name, backend_service, backend_method, timeout_ms, description, enabled) VALUES
-- Unified entry point for AI processing
('POST', '/v1/ai/process', 'ai', 'ai.v1.AIService', 'Process', 60000, 'Unified entry point for AI processing', 1),
-- Streaming AI response
('POST', '/v1/ai/process/stream', 'ai', 'ai.v1.AIService', 'ProcessStream', 60000, 'Streaming AI response', 1),
-- List available agents
('GET', '/v1/ai/agents', 'ai', 'ai.v1.AIService', 'ListAgents', 5000, 'List available AI agents', 1)
ON DUPLICATE KEY UPDATE 
    backend_name = VALUES(backend_name),
    backend_service = VALUES(backend_service),
    backend_method = VALUES(backend_method),
    timeout_ms = VALUES(timeout_ms),
    description = VALUES(description),
    enabled = VALUES(enabled);

