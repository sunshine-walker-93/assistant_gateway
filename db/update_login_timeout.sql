-- Update login route timeout to handle slow gRPC reflection queries
-- Execute: mysql -u assistant -p assistant_gateway_db < update_login_timeout.sql

USE assistant_gateway_db;

-- Increase login timeout from 5s to 15s to accommodate first-time reflection queries
UPDATE routes 
SET timeout_ms = 15000 
WHERE http_pattern = '/v1/user/login' AND backend_method = 'Login';

-- Also update other user routes to be safe
UPDATE routes 
SET timeout_ms = 10000 
WHERE backend_name = 'account' 
  AND http_pattern IN ('/v1/user/register', '/v1/user/get', '/v1/user/update')
  AND timeout_ms = 5000;

