-- 滑动窗口限流
-- KEYS[1]: 限流键，列表左端最新、右端最旧，元素为放行时刻的毫秒时间戳
-- ARGV[1]: 窗口内最多放行次数
-- ARGV[2]: 窗口长度（毫秒）
-- ARGV[3]: 操作，reserve 判定并记录，release 撤回一条记录
-- ARGV[4]: release 撤回的记录，即 reserve 返回的时间戳
-- reserve 返回写入窗口的时间戳，窗口已满时返回空串；release 返回撤回的记录数
--
-- 时间取 Redis 服务端时钟，共用同一份计数的实例不依赖各自进程的时区与时钟；
-- 读取、判定、写入、续期在同一次调用里完成，并发请求不会各自放行。
-- 只有放行才写入记录并续期，被拒的请求不改变窗口的结束时刻。
-- release 只撤回指定的那一条记录，不改动窗口的生存期。

redis.replicate_commands()

if ARGV[3] == 'release' then
    return redis.call('LREM', KEYS[1], 1, ARGV[4])
end

local maximum = tonumber(ARGV[1])
local window = tonumber(ARGV[2])
local now = redis.call('TIME')
local now_ms = tonumber(now[1]) * 1000 + math.floor(tonumber(now[2]) / 1000)

-- 记录数达到上限且最旧一条仍在窗口内：拒绝
if redis.call('LLEN', KEYS[1]) >= maximum then
    local oldest = tonumber(redis.call('LINDEX', KEYS[1], -1))
    if now_ms - oldest < window then
        return ''
    end
end

local stamp = string.format('%.0f', now_ms)
redis.call('LPUSH', KEYS[1], stamp)
redis.call('LTRIM', KEYS[1], 0, maximum - 1)
redis.call('PEXPIRE', KEYS[1], window)
return stamp
