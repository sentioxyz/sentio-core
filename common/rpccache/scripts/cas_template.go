package scripts

// CASTemplate ensures only one executor runs globally during the TTL.
// Usage:
// EVAL <script> 1 lock:my_job <token> <ttl_seconds>
// Return values:
// 1 -> lock acquired (caller should execute)
// 0 -> lock already held (skip)
const CASTemplate = `
local key = KEYS[1]
local token = ARGV[1]
local ttl = tonumber(ARGV[2])

if (not ttl) or ttl <= 0 then
  return redis.error_reply("ERR invalid ttl (must be positive integer)")
end

-- Fast path: try to create the key if absent
-- Using SET NX EX is atomic; still inside script for clarity.
local setResult = redis.call('SET', key, token, 'EX', ttl, 'NX')
if setResult then
  return 1  -- acquired
end
return 0      -- already held
`
