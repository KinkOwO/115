package main

import (
	"sync"
	"time"
)

// connectionSession owns resources whose lifetime is exactly one game
// connection. Gameplay state remains in worldSession; this type only owns the
// reader stop signal and periodic triggers used by the connection loop.
type connectionSession struct {
	done        chan struct{}
	mailChanges chan struct{}
	mailTicker  *time.Ticker
	dailyTicker *time.Ticker
	mineTicker  *time.Ticker
	moonTicker  *time.Ticker
	// forestBanner 是苏醒之森 Extreme「净化开始」横幅的精确到期信号。
	//
	// 为什么不能只靠 mineTicker（1 秒粒度）：N2568 让客户端启动
	// `[SEAMLESS LOADING] PREPARE_LEGION_ENTER_DUNGEON ... delay[4]`，
	// **进图帧列必须在它自己声明的 4 秒窗口内到达**（官服 22:05:41.514 →
	// 22:05:45.514 正好 4.000s）。晚到（我们此前是 4.1s + 1 秒 tick = 4.87s）
	// 客户端会退回非无缝路径，界面态留在「无缝加载中」，进图后**屏幕 UI 全丢**
	//（业主 2026-10-09 两次实机）。所以这里用一个精确定时器把到期时刻发回主
	// 循环——主循环独占 worldSession，状态推进仍然没有并发。
	forestBanner chan time.Time
	closeOnce    sync.Once
}

func newConnectionSession(moonEnabled bool) *connectionSession {
	s := &connectionSession{
		done:        make(chan struct{}),
		mailChanges: make(chan struct{}, 1),
		mailTicker:  time.NewTicker(2 * time.Second),
		dailyTicker: time.NewTicker(30 * time.Second),
		mineTicker:  time.NewTicker(time.Second),
		// 缓冲 1：即使主循环正在忙，定时器也不会阻塞在发送上。
		forestBanner: make(chan time.Time, 1),
	}
	if moonEnabled {
		s.moonTicker = time.NewTicker(250 * time.Millisecond)
	}
	return s
}

// scheduleForestBanner 在 delay 之后把「净化开始演出结束」信号交给主循环。
// 只发送时间戳、不碰任何会话状态（worldSession 由主循环独占）。
func (s *connectionSession) scheduleForestBanner(delay time.Duration) {
	if s == nil {
		return
	}
	go func() {
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-s.done:
			return
		}
		select {
		case s.forestBanner <- time.Now():
		case <-s.done:
		}
	}()
}

func (s *connectionSession) moonTicks() <-chan time.Time {
	if s == nil || s.moonTicker == nil {
		return nil
	}
	return s.moonTicker.C
}

func (s *connectionSession) close() {
	if s == nil {
		return
	}
	s.closeOnce.Do(func() {
		close(s.done)
		s.mailTicker.Stop()
		s.dailyTicker.Stop()
		s.mineTicker.Stop()
		if s.moonTicker != nil {
			s.moonTicker.Stop()
		}
	})
}
