import sqlite3, os

home = os.path.expanduser('~')

# CCSwitch DB
cc = sqlite3.connect(os.path.join(home, '.cc-switch', 'cc-switch.db'))
print('=== CC-Switch DB (proxy_request_logs) 2026-07-01 ===')
rows = cc.execute("""
    SELECT date(created_at, 'unixepoch', 'localtime') as d,
           app_type, model,
           SUM(input_tokens), SUM(output_tokens),
           SUM(cache_read_tokens), SUM(cache_creation_tokens),
           SUM(total_cost_usd)
    FROM proxy_request_logs
    WHERE date(created_at, 'unixepoch', 'localtime') = '2026-07-01'
      AND app_type = 'claude'
      AND data_source = 'proxy'
      AND status_code >= 200 AND status_code < 300
    GROUP BY d, app_type, model
    ORDER BY SUM(input_tokens+output_tokens) DESC
""")
for r in rows:
    cost = float(r[7]) if r[7] else 0
    print(f'  {r[2]:35s} in={r[3]:>10} out={r[4]:>10} cache_r={r[5]:>10} cache_c={r[6]:>10} cost={cost:.6f}')

total = cc.execute("""
    SELECT COALESCE(SUM(input_tokens),0), COALESCE(SUM(output_tokens),0),
           COALESCE(SUM(cache_read_tokens),0), COALESCE(SUM(cache_creation_tokens),0),
           COALESCE(SUM(total_cost_usd),0)
    FROM proxy_request_logs
    WHERE date(created_at, 'unixepoch', 'localtime') = '2026-07-01'
      AND app_type = 'claude'
      AND data_source = 'proxy'
      AND status_code >= 200 AND status_code < 300
""").fetchone()
print(f'  --- 总计: in={total[0]:>10} out={total[1]:>10} cache_r={total[2]:>10} cache_c={total[3]:>10} cost=${float(total[4]):.4f}')

# Also check rollups
print()
print('=== CC-Switch DB (usage_daily_rollups) 2026-07-01 ===')
rollup = cc.execute("""
    SELECT app_type, model, SUM(input_tokens), SUM(output_tokens),
           SUM(cache_read_tokens), SUM(cache_creation_tokens),
           SUM(total_cost_usd)
    FROM usage_daily_rollups
    WHERE date = '2026-07-01'
    GROUP BY app_type, model
    ORDER BY SUM(input_tokens+output_tokens) DESC
""")
for r in rollup:
    cost = float(r[6]) if r[6] else 0
    print(f'  {r[1]:35s} in={r[2]:>10} out={r[3]:>10} cache_r={r[4]:>10} cache_c={r[5]:>10} cost={cost:.6f}')

rt = cc.execute("""
    SELECT COALESCE(SUM(input_tokens),0), COALESCE(SUM(output_tokens),0),
           COALESCE(SUM(cache_read_tokens),0), COALESCE(SUM(cache_creation_tokens),0),
           COALESCE(SUM(total_cost_usd),0)
    FROM usage_daily_rollups WHERE date = '2026-07-01'
""").fetchone()
print(f'  --- 总计: in={rt[0]:>10} out={rt[1]:>10} cache_r={rt[2]:>10} cache_c={rt[3]:>10} cost=${float(rt[4]):.4f}')

cc.close()

# Token Dashboard DB
print()
print('=== Token Dashboard DB (daily_usage) 2026-07-01 ===')
td = sqlite3.connect(os.path.join(home, '.token-dashboard', 'td.db'))
rows = td.execute("""
    SELECT source, model, input_tokens, output_tokens,
           cache_read_tokens, cache_creation_tokens,
           total_tokens, cost_usd
    FROM daily_usage
    WHERE usage_date = '2026-07-01'
    ORDER BY total_tokens DESC
""")
for r in rows:
    tot = r[6]
    if tot == 0 and r[2] == 0 and r[3] == 0:
        print(f'  {r[1]:35s} tot={tot:>12} (zero-token record)')
    else:
        print(f'  {r[1]:35s} in={r[2]:>10} out={r[3]:>10} cache_r={r[4]:>10} cache_c={r[5]:>10} tot={tot:>12} cost=${r[7]:.4f}')

totals = td.execute("""
    SELECT COALESCE(SUM(input_tokens),0), COALESCE(SUM(output_tokens),0),
           COALESCE(SUM(cache_read_tokens),0), COALESCE(SUM(cache_creation_tokens),0),
           COALESCE(SUM(total_tokens),0), COALESCE(SUM(cost_usd),0)
    FROM daily_usage WHERE usage_date = '2026-07-01'
""").fetchone()
print(f'  --- 总计: in={totals[0]:>10} out={totals[1]:>10} cache_r={totals[2]:>10} cache_c={totals[3]:>10} tot={totals[4]:>12} cost=${totals[5]:.4f}')

td.close()
