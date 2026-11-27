-- 滑动窗口限流
-- KEYS[1]: 限流键，列表左端最新、右端最旧，元素为放行时刻的毫秒时间戳
-- ARGV[1]: 窗口内最多放行次数
-- ARGV[2]: 窗口长度（毫秒）
-- ARGV[3]: 操作，take 判定并记录，peek 只判定，record 只记录
-- 返回 1 表示放行，0 表示拒绝
--
-- 时间取 Redis 服务端时钟，共用同一份计数的实例不依赖各自进程的时区与时钟；
-- 读取、判定、写入、续期在同一次调用里完成，并发请求不会各自放行。
-- 只有放行才写入记录并续期，被拒的请求不改变窗口的结束时刻。

redis.replicate_commands()

local maximum = tonumber(ARGV[1])
local window = tonumber(ARGV[2])
local now = redis.call('TIME')
local now_ms = tonumber(now[1]) * 1000 + math.floor(tonumber(now[2]) / 1000)

local allowed = 1
if ARGV[3] ~= 'record' and redis.call('LLEN', KEYS[1]) >= maximum then
    local oldest = tonumber(redis.call('LINDEX', KEYS[1], -1))
    if now_ms - oldest < window then
        allowed = 0
    end
end

if allowed == 1 and ARGV[3] ~= 'peek' then
    redis.call('LPUSH', KEYS[1], string.format('%.0f', now_ms))
    redis.call('LTRIM', KEYS[1], 0, maximum - 1)
    redis.call('PEXPIRE', KEYS[1], window)
end

return allowed
