-- 添加用户管理相关的路由配置
-- 执行方式：mysql -u assistant -p assistant_gateway_db < add_user_routes.sql
-- 或者在 MySQL 客户端中执行

USE assistant_gateway_db;

-- 添加用户注册路由
INSERT INTO routes (http_method, http_pattern, backend_name, backend_service, backend_method, timeout_ms, description, enabled) VALUES
('POST', '/v1/user/register', 'account', 'user.v1.UserService', 'Register', 5000, 'User registration route', 1)
ON DUPLICATE KEY UPDATE 
    backend_name = VALUES(backend_name),
    backend_service = VALUES(backend_service),
    backend_method = VALUES(backend_method),
    timeout_ms = VALUES(timeout_ms),
    description = VALUES(description),
    enabled = VALUES(enabled);

-- 添加获取用户信息路由
INSERT INTO routes (http_method, http_pattern, backend_name, backend_service, backend_method, timeout_ms, description, enabled) VALUES
('POST', '/v1/user/get', 'account', 'user.v1.UserService', 'GetUser', 5000, 'Get user by ID route', 1)
ON DUPLICATE KEY UPDATE 
    backend_name = VALUES(backend_name),
    backend_service = VALUES(backend_service),
    backend_method = VALUES(backend_method),
    timeout_ms = VALUES(timeout_ms),
    description = VALUES(description),
    enabled = VALUES(enabled);

-- 添加更新用户信息路由
INSERT INTO routes (http_method, http_pattern, backend_name, backend_service, backend_method, timeout_ms, description, enabled) VALUES
('POST', '/v1/user/update', 'account', 'user.v1.UserService', 'UpdateUser', 5000, 'Update user info route', 1)
ON DUPLICATE KEY UPDATE 
    backend_name = VALUES(backend_name),
    backend_service = VALUES(backend_service),
    backend_method = VALUES(backend_method),
    timeout_ms = VALUES(timeout_ms),
    description = VALUES(description),
    enabled = VALUES(enabled);

