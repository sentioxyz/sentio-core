package requestlimiter

const acquireTemplate = `
-- acquire.lua

local user_key = KEYS[1]
local project_key = KEYS[2]
local ip_key = KEYS[3]
local tier_key = KEYS[4]

local current_timestamp = tonumber(ARGV[1])
local expired_timestamp = tonumber(ARGV[2])
local limiter_id = ARGV[3]
local user_quota = tonumber(ARGV[4])
local project_quota = tonumber(ARGV[5])
local ip_quota = tonumber(ARGV[6])
local tier_quota = tonumber(ARGV[7])

local function not_empty(s)
  return s ~= nil and #s > 0
end

-- Remove expired members
if not_empty(user_key) then
	redis.call('ZREMRANGEBYSCORE', user_key, '-inf', expired_timestamp)
end
if not_empty(project_key) then
	redis.call('ZREMRANGEBYSCORE', project_key, '-inf', expired_timestamp)
end
if not_empty(ip_key) then
	redis.call('ZREMRANGEBYSCORE', ip_key, '-inf', expired_timestamp)
end
if not_empty(tier_key) then
	redis.call('ZREMRANGEBYSCORE', tier_key, '-inf', expired_timestamp)
end
-- Check if has reached quota


local current_user = redis.call('ZCARD', user_key)
local current_project = redis.call('ZCARD', project_key)
local current_ip = redis.call('ZCARD', ip_key)
local current_tier = redis.call('ZCARD', tier_key)

if not_empty(user_key) and user_quota > 0 then
	if current_user >= user_quota then
		return 'user_reached_quota:' .. tostring(current_user) .. '/' .. tostring(user_quota)
	end
end
if not_empty(project_key) and project_quota > 0 then
	if current_project >= project_quota then
		return 'project_reached_quota:' .. tostring(current_project) .. '/' .. tostring(project_quota)
	end
end
if not_empty(ip_key) and ip_quota > 0 then
	if current_ip >= ip_quota then
		return 'ip_reached_quota:' .. tostring(current_ip) .. '/' .. tostring(ip_quota)
	end
end
if not_empty(tier_key) and tier_quota > 0 then
	if current_tier >= tier_quota then
		return 'tier_reached_quota:' .. tostring(current_tier) .. '/' .. tostring(tier_quota)
	end
end

-- Acquire
if not_empty(user_key) then
	redis.call('ZADD', user_key, current_timestamp, limiter_id)
end
if not_empty(project_key) then
	redis.call('ZADD', project_key, current_timestamp, limiter_id)
end
if not_empty(ip_key) then
	redis.call('ZADD', ip_key, current_timestamp, limiter_id)
end
if not_empty(tier_key) then
	redis.call('ZADD', tier_key, current_timestamp, limiter_id)
end

return 'ok:' .. tostring(current_user+1) .. '/' .. tostring(current_project+1) .. '/' .. tostring(current_ip+1) .. '/' .. tostring(current_tier+1)
`

const releaseTemplate = `
-- release.lua

local user_key = KEYS[1]
local project_key = KEYS[2]
local ip_key = KEYS[3]
local tier_key = KEYS[4]

local limiter_id = ARGV[1]

local function not_empty(s)
  return s ~= nil and #s > 0
end

if not_empty(user_key) then
	redis.call('ZREM', user_key, limiter_id)
end
if not_empty(project_key) then
	redis.call('ZREM', project_key, limiter_id)
end
if not_empty(ip_key) then
	redis.call('ZREM', ip_key, limiter_id)
end
if not_empty(tier_key) then
	redis.call('ZREM', tier_key, limiter_id)
end

local current_user = redis.call('ZCARD', user_key)
local current_project = redis.call('ZCARD', project_key)
local current_ip = redis.call('ZCARD', ip_key)
local current_tier = redis.call('ZCARD', tier_key)

return 'ok:' .. tostring(current_user) .. '/' .. tostring(current_project) .. '/' .. tostring(current_ip) .. '/' .. tostring(current_tier)
`
