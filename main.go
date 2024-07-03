package main

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"

	"redislock/lock"
)

func main() {
	ctx := context.Background()

	redisClient := redis.NewClient(&redis.Options{
		Addr:         "127.0.0.1:6379",
		DB:           0,
		ReadTimeout:  time.Second * 3,
		WriteTimeout: time.Second * 3,
	})

	// 实例化锁
	redisLock := lock.NewRedisLock(redisClient, "test_lock", 10)

	// 设置看门狗最大执行次数 默认无限次执行 设置为0则不使用看门狗 避免死锁
	redisLock.SetWatchDagExecCount(10)

	// 加锁
	redisLock.Lock(ctx)
	// 循环加锁
	// redisLock.LoopLock(ctx, 1)

	// do something

	//解锁
	redisLock.Unlock(ctx)
}
