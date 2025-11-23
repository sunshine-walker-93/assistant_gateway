-- Schema for Assistant Gateway Service
-- Database: assistant_gateway_db

-- Create database
CREATE DATABASE IF NOT EXISTS assistant_gateway_db
  DEFAULT CHARACTER SET utf8mb4
  DEFAULT COLLATE utf8mb4_unicode_ci;

USE assistant_gateway_db;

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

