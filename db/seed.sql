-- Database initialization script for Assistant Gateway
-- This script creates the database, schema, and inserts initial test data.
-- Execute this script manually as root user to initialize the database.

-- ============================================
-- Database Creation
-- ============================================

CREATE DATABASE IF NOT EXISTS assistant_gateway_db
  DEFAULT CHARACTER SET utf8mb4
  DEFAULT COLLATE utf8mb4_unicode_ci;

-- ============================================
-- Grant Permissions
-- ============================================
-- Grant all privileges on assistant_gateway_db to assistant user
-- This is required for the gateway services to access the database
GRANT ALL PRIVILEGES ON assistant_gateway_db.* TO 'assistant'@'%';
FLUSH PRIVILEGES;

USE assistant_gateway_db;

-- ============================================
-- Schema Creation (from schema.sql)
-- ============================================

-- 后端服务表
-- 用于存储静态服务地址（因为没有服务注册中心）
CREATE TABLE IF NOT EXISTS backends (
    id INT UNSIGNED PRIMARY KEY AUTO_INCREMENT COMMENT '主键ID',
    name VARCHAR(100) UNIQUE NOT NULL COMMENT '后端服务名称，唯一标识',
    addr VARCHAR(255) NOT NULL COMMENT '后端服务地址，格式: host:port',
    description VARCHAR(500) COMMENT '服务描述',
    enabled TINYINT(1) DEFAULT 1 COMMENT '是否启用，1=启用，0=禁用',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    INDEX idx_name (name),
    INDEX idx_enabled (enabled)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='后端服务配置表';

-- 路由配置表
CREATE TABLE IF NOT EXISTS routes (
    id INT UNSIGNED PRIMARY KEY AUTO_INCREMENT COMMENT '主键ID',
    http_method VARCHAR(10) NOT NULL COMMENT 'HTTP方法，如 GET, POST, PUT, DELETE',
    http_pattern VARCHAR(500) NOT NULL COMMENT 'HTTP路径模式，如 /v1/user/login',
    backend_name VARCHAR(100) NOT NULL COMMENT '关联的后端服务名称',
    backend_service VARCHAR(255) NOT NULL COMMENT 'gRPC服务全名，如 user.v1.UserService',
    backend_method VARCHAR(100) NOT NULL COMMENT 'gRPC方法名，如 Login',
    timeout_ms INT UNSIGNED DEFAULT 5000 COMMENT '超时时间（毫秒）',
    description VARCHAR(500) COMMENT '路由描述',
    enabled TINYINT(1) DEFAULT 1 COMMENT '是否启用，1=启用，0=禁用',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    UNIQUE KEY uk_method_pattern (http_method, http_pattern),
    FOREIGN KEY (backend_name) REFERENCES backends(name) ON DELETE RESTRICT ON UPDATE CASCADE,
    INDEX idx_backend_name (backend_name),
    INDEX idx_enabled (enabled)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='路由配置表';

-- 配置变更历史表（审计用）
CREATE TABLE IF NOT EXISTS config_history (
    id BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT COMMENT '主键ID',
    config_type ENUM('backend', 'route') NOT NULL COMMENT '配置类型',
    config_id INT UNSIGNED COMMENT '关联的配置ID（backend或route的id）',
    operation ENUM('CREATE', 'UPDATE', 'DELETE') NOT NULL COMMENT '操作类型',
    old_value JSON COMMENT '变更前的配置（JSON格式）',
    new_value JSON COMMENT '变更后的配置（JSON格式）',
    operator VARCHAR(100) COMMENT '操作人（未来用于权限控制）',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT '变更时间',
    INDEX idx_config_type_id (config_type, config_id),
    INDEX idx_created_at (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='配置变更历史表';

-- ============================================
-- Initial Test Data
-- ============================================

-- 插入测试后端服务（account 服务）
-- 注意：在 Docker 环境中，使用容器名作为地址
-- 在本地开发环境中，可能需要使用 127.0.0.1:50051
INSERT INTO backends (name, addr, description, enabled) VALUES
('account', 'assistant-account-app:50051', 'Account service for user management', 1)
ON DUPLICATE KEY UPDATE 
    addr = VALUES(addr),
    description = VALUES(description),
    enabled = VALUES(enabled);

-- 插入测试路由（用户登录）
INSERT INTO routes (http_method, http_pattern, backend_name, backend_service, backend_method, timeout_ms, description, enabled) VALUES
('POST', '/v1/user/login', 'account', 'user.v1.UserService', 'Login', 5000, 'User login route', 1)
ON DUPLICATE KEY UPDATE 
    backend_name = VALUES(backend_name),
    backend_service = VALUES(backend_service),
    backend_method = VALUES(backend_method),
    timeout_ms = VALUES(timeout_ms),
    description = VALUES(description),
    enabled = VALUES(enabled);

-- 插入更多用户管理路由
INSERT INTO routes (http_method, http_pattern, backend_name, backend_service, backend_method, timeout_ms, description, enabled) VALUES
('POST', '/v1/user/register', 'account', 'user.v1.UserService', 'Register', 5000, 'User registration route', 1),
('POST', '/v1/user/get', 'account', 'user.v1.UserService', 'GetUser', 5000, 'Get user by ID route', 1),
('POST', '/v1/user/update', 'account', 'user.v1.UserService', 'UpdateUser', 5000, 'Update user info route', 1)
ON DUPLICATE KEY UPDATE 
    backend_name = VALUES(backend_name),
    backend_service = VALUES(backend_service),
    backend_method = VALUES(backend_method),
    timeout_ms = VALUES(timeout_ms),
    description = VALUES(description),
    enabled = VALUES(enabled);

