package lock

import "context"

const (
	_                 = iota
	BreakStatusNormal // 正常关闭
	BreakStatusGtMax  // 超过最大循环次数关闭
)

type While struct {
	Ctx         context.Context
	Cancel      context.CancelFunc
	MaxWorkNum  int
	BreakStatus int // break原因
}

func NewWhile(maxWorkNum int) *While {
	ctx, cancel := context.WithCancel(context.Background())
	return &While{
		Ctx:        ctx,
		Cancel:     cancel,
		MaxWorkNum: maxWorkNum,
	}
}

func (w *While) For(f func()) {
	var count int
loop:
	for {
		// 用户break
		select {
		case <-w.Ctx.Done():
			break loop
		default:
		}

		// 执行事件
		f()

		// 判断执行次数是否超过最大次数
		count++
		if count >= w.MaxWorkNum {
			w.BreakStatus = BreakStatusGtMax
			break
		}
	}
}

func (w *While) Break() {
	w.BreakStatus = BreakStatusNormal
	w.Cancel()
}

func (w *While) IsNormal() bool {
	return w.BreakStatus == BreakStatusNormal
}
