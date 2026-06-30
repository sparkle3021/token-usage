-- 清空业务数据（保留 app_config、parse_cache、collection_runs 等系统表）
DELETE FROM daily_usage;
DELETE FROM session_usage;
DELETE FROM time_usage;
DELETE FROM hour_usage;
-- 清除 CC-Switch checkpoint，下次会全量重新同步
DELETE FROM app_config WHERE key LIKE 'cc_switch_cursor_%';
DELETE FROM app_config WHERE key LIKE 'cc_switch_rollup_%';
-- 保留 parse_cache 和 collection_runs 不受影响
