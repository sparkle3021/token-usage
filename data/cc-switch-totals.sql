-- 查询 CC-Switch 两个表的 token 总量（需在 cc-switch.db 中执行）
-- 用法: sqlite3 ~/.cc-switch/cc-switch.db < data/cc-switch-totals.sql

-- 1. proxy_request_logs 总量（仅当月数据）
SELECT '=== proxy_request_logs ===' as info;
SELECT COUNT(*) as total_rows,
       SUM(input_tokens) as total_input,
       SUM(output_tokens) as total_output,
       SUM(cache_read_tokens) as total_cache_read,
       SUM(cache_creation_tokens) as total_cache_creation,
       SUM(input_tokens + output_tokens + cache_read_tokens + cache_creation_tokens) as total_tokens
FROM proxy_request_logs
WHERE app_type = 'claude'
  AND data_source = 'proxy'
  AND status_code >= 200 AND status_code < 300;

-- 2. usage_daily_rollups 总量（历史汇总数据）
SELECT '=== usage_daily_rollups ===' as info;
SELECT COUNT(*) as total_rows,
       SUM(input_tokens) as total_input,
       SUM(output_tokens) as total_output,
       SUM(cache_read_tokens) as total_cache_read,
       SUM(cache_creation_tokens) as total_cache_creation,
       SUM(input_tokens + output_tokens + cache_read_tokens + cache_creation_tokens) as total_tokens
FROM usage_daily_rollups;

-- 3. 各模型占比（用于核对导入后是否一致）
SELECT '=== proxy_request_logs by model ===' as info;
SELECT model,
       COUNT(*) as rows,
       SUM(input_tokens + output_tokens + cache_read_tokens + cache_creation_tokens) as total_tokens
FROM proxy_request_logs
WHERE app_type = 'claude'
  AND data_source = 'proxy'
  AND status_code >= 200 AND status_code < 300
GROUP BY model
ORDER BY total_tokens DESC;
