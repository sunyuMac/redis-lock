package lock

import (
	"context"
	_ "embed"
	"fmt"
	"math/rand"
	"time"

	"github.com/redis/go-redis/v9"
)

//go:embed lua/unlock.lua
var unLockScript string

//go:embed lua/watchdog.lua
var watchDogScript string

const (
	// lockMaxLoopNum 加锁最大循环数量
	lockMaxLoopNum = 1000
	// watchDogDefaultExecCount 看门狗默认执行次数 无限次
	watchDogDefaultExecCount = -1
)

type RedisLock struct {
	// redis客户端，调用者注入进来
	client *redis.Client
	// redis key
	key string
	// 锁的value，随机数，防止被其他协程开锁
	value string
	// 过期时间
	expire int
	// 解锁通知channel
	unlockCh chan struct{}

	// 看门狗执行次数
	watchDogExecCount int
}

// NewRedisLock 初始化redis分布式锁
func NewRedisLock(client *redis.Client, key string, expire int) *RedisLock {
	return &RedisLock{
		key:               key,
		expire:            expire,
		value:             fmt.Sprintf("%d", Random(100000000, 999999999)), // 随机值作为锁的值
		client:            client,
		unlockCh:          make(chan struct{}, 0),
		watchDogExecCount: watchDogDefaultExecCount,
	}
}

// SetWatchDagExecCount 设置看门狗的最大执行次数
func (l *RedisLock) SetWatchDagExecCount(count int) {
	l.watchDogExecCount = count
}

// getScript
func (l *RedisLock) getScript(ctx context.Context, script string) string {
	scriptString, _ := l.client.ScriptLoad(ctx, script).Result()
	return scriptString
}

// Lock 加锁
func (l *RedisLock) Lock(ctx context.Context) bool {
	ok, _ := l.client.SetNX(ctx, l.key, l.value, time.Duration(l.expire)*time.Second).Result()
	if ok && l.watchDogExecCount != 0 {
		go l.watchDog(ctx)
	}
	return ok
}

// LoopLock 循环加锁
func (l *RedisLock) LoopLock(
	ctx context.Context,
	sleepTime int, // 循环等待时间（毫秒）
) bool {
	t := time.NewTicker(time.Duration(sleepTime) * time.Millisecond)
	w := NewWhile(lockMaxLoopNum)
	w.For(func() {
		if l.Lock(ctx) {
			t.Stop()
			w.Break()
		} else {
			<-t.C
		}
	})
	if !w.IsNormal() {
		return false
	}
	return true
}

// Unlock 解锁
func (l *RedisLock) Unlock(ctx context.Context) bool {
	args := []interface{}{
		l.value, // 脚本中的argv
	}
	flag, _ := l.client.EvalSha(ctx, l.getScript(ctx, unLockScript), []string{l.key}, args...).Result()
	// 关闭看门狗
	close(l.unlockCh)
	return lockRes(flag.(int64))
}

// watchDog 看门狗，锁到期前自动续期
func (l *RedisLock) watchDog(ctx context.Context) {
	// 创建一个定时器NewTicker，每过期时间的4分之3触发一次
	loopTime := time.Duration(l.expire*1e3*3/4) * time.Millisecond
	expTicker := time.NewTicker(loopTime)
	execCount := 0
loop:
	// 确认锁与锁续期打包原子化
	for {
		select {
		case <-expTicker.C:
			args := []interface{}{
				l.value,
				l.expire,
			}
			res, err := l.client.EvalSha(ctx, l.getScript(ctx, watchDogScript), []string{l.key}, args...).Result()
			if err != nil {
				fmt.Println("watchDog error", err)
				return
			}
			r, ok := res.(int64)
			if !ok || r == 0 {
				return
			}
			// 判断看门狗是否到执行次数限制
			if l.watchDogExecCount != watchDogDefaultExecCount {
				execCount++
				if execCount >= l.watchDogExecCount {
					break loop
				}
			}
		case <-l.unlockCh: // 任务完成后用户解锁通知看门狗退出
			return
		}
	}
}

func lockRes(flag int64) bool {
	if flag > 0 {
		return true
	}
	return false
}

func Random(min, max int64) int64 {
	rand.Seed(time.Now().UnixNano())
	return rand.Int63n(max-min+1) + min
}
