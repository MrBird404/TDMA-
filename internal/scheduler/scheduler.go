package scheduler

import (
	"fmt"
	"sync"
	"time"
)

// 时隙状态
type SlotStatus struct {
	SlotID     int
	NodeID     string
	Status     string // "FREE", "ASSIGNED", "BUSY"
	StartTime  time.Time
	Duration   time.Duration
	FragmentID uint32
}

// TDMA调度器
type TDMAScheduler struct {
	mu           sync.RWMutex
	slots        map[int]*SlotStatus
	totalSlots   int
	slotDuration time.Duration
	currentSlot  int
	startTime    time.Time
}

// 创建新的TDMA调度器
func NewTDMAScheduler(totalSlots int, slotDuration time.Duration) *TDMAScheduler {
	scheduler := &TDMAScheduler{
		slots:        make(map[int]*SlotStatus),
		totalSlots:   totalSlots,
		slotDuration: slotDuration,
		currentSlot:  0,
		startTime:    time.Now(),
	}

	// 初始化所有时隙为FREE状态
	for i := 0; i < totalSlots; i++ {
		scheduler.slots[i] = &SlotStatus{
			SlotID:   i,
			Status:   "FREE",
			Duration: slotDuration,
		}
	}

	return scheduler
}

// 分配时隙
func (s *TDMAScheduler) AllocateTimeSlot(nodeID string, priority int) (slotID int, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 首先检查节点是否已经有分配的时隙
	for i := 0; i < s.totalSlots; i++ {
		if s.slots[i].NodeID == nodeID && s.slots[i].Status == "ASSIGNED" {
			// 如果已分配的时隙仍然有效，直接返回
			if time.Since(s.slots[i].StartTime) < s.slotDuration*10 {
				return i, nil
			}
			// 如果时隙已过期，释放它
			s.slots[i].Status = "FREE"
			s.slots[i].NodeID = ""
		}
	}

	// 优先分配当前时隙或下一个时隙
	currentSlot := s.currentSlot
	nextSlot := (currentSlot + 1) % s.totalSlots

	// 检查当前时隙是否可用
	if s.slots[currentSlot].Status == "FREE" {
		s.slots[currentSlot].NodeID = nodeID
		s.slots[currentSlot].Status = "ASSIGNED"
		s.slots[currentSlot].StartTime = time.Now()
		return currentSlot, nil
	}

	// 检查下一个时隙是否可用
	if s.slots[nextSlot].Status == "FREE" {
		s.slots[nextSlot].NodeID = nodeID
		s.slots[nextSlot].Status = "ASSIGNED"
		s.slots[nextSlot].StartTime = time.Now()
		return nextSlot, nil
	}

	// 如果当前和下一个时隙都不可用，查找最近的可用时隙
	for offset := 2; offset < s.totalSlots; offset++ {
		slotID := (currentSlot + offset) % s.totalSlots
		if s.slots[slotID].Status == "FREE" {
			s.slots[slotID].NodeID = nodeID
			s.slots[slotID].Status = "ASSIGNED"
			s.slots[slotID].StartTime = time.Now()
			return slotID, nil
		}
	}

	// 如果没有可用时隙，尝试重用最旧的已分配时隙
	oldestSlot := -1
	oldestTime := time.Now()
	for i := 0; i < s.totalSlots; i++ {
		if s.slots[i].Status == "ASSIGNED" {
			if s.slots[i].StartTime.Before(oldestTime) {
				oldestTime = s.slots[i].StartTime
				oldestSlot = i
			}
		}
	}

	if oldestSlot != -1 && time.Since(oldestTime) > s.slotDuration*5 {
		s.slots[oldestSlot].NodeID = nodeID
		s.slots[oldestSlot].Status = "ASSIGNED"
		s.slots[oldestSlot].StartTime = time.Now()
		return oldestSlot, nil
	}

	return -1, fmt.Errorf("没有可用的时隙")
}

// 分配连续时隙
func (s *TDMAScheduler) AllocateConsecutiveSlots(nodeID string, count int) ([]int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var slotIDs []int

	// 查找连续可用时隙
	for i := 0; i <= s.totalSlots-count; i++ {
		available := true
		for j := 0; j < count; j++ {
			if s.slots[i+j].Status != "FREE" {
				available = false
				break
			}
		}

		if available {
			for j := 0; j < count; j++ {
				s.slots[i+j].NodeID = nodeID
				s.slots[i+j].Status = "ASSIGNED"
				s.slots[i+j].StartTime = time.Now()
				slotIDs = append(slotIDs, i+j)
			}
			return slotIDs, nil
		}
	}

	return nil, fmt.Errorf("没有足够的连续时隙")
}

// 释放时隙
func (s *TDMAScheduler) ReleaseTimeSlot(slotID int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if slotID < 0 || slotID >= s.totalSlots {
		return fmt.Errorf("无效的时隙ID")
	}

	s.slots[slotID].Status = "FREE"
	s.slots[slotID].NodeID = ""
	s.slots[slotID].FragmentID = 0

	return nil
}

// 获取调度表
func (s *TDMAScheduler) GetSchedule() map[int]string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	schedule := make(map[int]string)
	for slotID, status := range s.slots {
		if status.Status == "ASSIGNED" {
			schedule[slotID] = status.NodeID
		}
	}

	return schedule
}

// 更新优先级
func (s *TDMAScheduler) UpdatePriority(nodeID string, newPriority int) error {
	// 这里可以实现基于优先级的时隙重新分配
	// 简化实现，只记录优先级
	return nil
}

// 获取当前时隙
func (s *TDMAScheduler) GetCurrentSlot() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.currentSlot
}

// 获取时隙状态
func (s *TDMAScheduler) GetSlotStatus(slotID int) (*SlotStatus, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if slotID < 0 || slotID >= s.totalSlots {
		return nil, fmt.Errorf("无效的时隙ID")
	}

	return s.slots[slotID], nil
}

// 启动调度器
func (s *TDMAScheduler) Start() error {
	go s.scheduleLoop()
	return nil
}

// 停止调度器
func (s *TDMAScheduler) Stop() error {
	// 停止调度循环
	return nil
}

// 调度循环
func (s *TDMAScheduler) scheduleLoop() {
	ticker := time.NewTicker(s.slotDuration)
	defer ticker.Stop()

	for range ticker.C {
		s.mu.Lock()
		s.currentSlot = (s.currentSlot + 1) % s.totalSlots
		s.mu.Unlock()
	}
}

// 获取下一个可用时隙
func (s *TDMAScheduler) GetNextAvailableSlot() (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for i := 0; i < s.totalSlots; i++ {
		if s.slots[i].Status == "FREE" {
			return i, nil
		}
	}

	return -1, fmt.Errorf("没有可用的时隙")
}

// 打印调度状态
func (s *TDMAScheduler) PrintStatus() {
	s.mu.RLock()
	defer s.mu.RUnlock()

	fmt.Printf("=== TDMA调度器状态 ===\n")
	fmt.Printf("当前时隙: %d\n", s.currentSlot)
	fmt.Printf("总时隙数: %d\n", s.totalSlots)
	fmt.Printf("时隙持续时间: %v\n", s.slotDuration)
	fmt.Printf("调度表:\n")

	for i := 0; i < s.totalSlots; i++ {
		status := s.slots[i]
		fmt.Printf("  时隙 %d: %s (节点: %s)\n", i, status.Status, status.NodeID)
	}
	fmt.Printf("==================\n")
}

// 默认时隙持续时间和总时隙数
var DefaultSlotDuration = time.Second
var DefaultTotalSlots = 10
