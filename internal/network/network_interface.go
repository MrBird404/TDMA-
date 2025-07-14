package network

import (
	"fmt"
	"net"
	"sync"
	"time"
	"tdma-network/pkg/protocol"
)

// 连接状态
type ConnectionStatus struct {
	Connected bool
	Address   string
	LastSeen  time.Time
}

// 网络接口层
type NetworkInterface struct {
	conn         net.Conn
	address      string
	connected    bool
	mu           sync.RWMutex
	timeout      time.Duration
	fragmentTimeout time.Duration
}

// 创建新的网络接口
func NewNetworkInterface() *NetworkInterface {
	return &NetworkInterface{
		timeout:        5 * time.Second,
		fragmentTimeout: 10 * time.Second,
	}
}

// 连接到目标地址
func (ni *NetworkInterface) Connect(target string) error {
	ni.mu.Lock()
	defer ni.mu.Unlock()
	
	conn, err := net.DialTimeout("tcp", target, ni.timeout)
	if err != nil {
		return fmt.Errorf("连接失败: %v", err)
	}
	
	ni.conn = conn
	ni.address = target
	ni.connected = true
	
	fmt.Printf("已连接到 %s\n", target)
	return nil
}

// 断开连接
func (ni *NetworkInterface) Disconnect() error {
	ni.mu.Lock()
	defer ni.mu.Unlock()
	
	if ni.conn != nil {
		ni.conn.Close()
		ni.conn = nil
	}
	
	ni.connected = false
	fmt.Printf("已断开连接\n")
	return nil
}

// 发送TDMA帧
func (ni *NetworkInterface) SendFrame(frame *protocol.TDMAFrame, target string) error {
	ni.mu.RLock()
	defer ni.mu.RUnlock()
	
	if !ni.connected {
		return fmt.Errorf("未连接")
	}
	
	// 序列化帧
	data, err := frame.Serialize()
	if err != nil {
		return fmt.Errorf("序列化失败: %v", err)
	}
	
	// 发送数据
	_, err = ni.conn.Write(data)
	if err != nil {
		return fmt.Errorf("发送失败: %v", err)
	}
	
	fmt.Printf("发送帧: %s\n", frame.String())
	return nil
}

// 发送多个分片
func (ni *NetworkInterface) SendFragments(fragments []*protocol.TDMAFrame, target string) error {
	for i, fragment := range fragments {
		err := ni.SendFrame(fragment, target)
		if err != nil {
			return fmt.Errorf("发送分片 %d 失败: %v", i, err)
		}
		
		// 分片间延迟
		if i < len(fragments)-1 {
			time.Sleep(100 * time.Millisecond)
		}
	}
	
	return nil
}

// 接收TDMA帧
func (ni *NetworkInterface) ReceiveFrame() (*protocol.TDMAFrame, error) {
	ni.mu.RLock()
	defer ni.mu.RUnlock()
	
	if !ni.connected {
		return nil, fmt.Errorf("未连接")
	}
	
	// 设置读取超时
	ni.conn.SetReadDeadline(time.Now().Add(ni.timeout))
	
	// 读取帧头以确定长度
	header := make([]byte, 8)
	_, err := ni.conn.Read(header)
	if err != nil {
		return nil, fmt.Errorf("读取帧头失败: %v", err)
	}
	
	// 检查帧头
	if string(header) != string(protocol.FRAME_HEADER[:]) {
		return nil, fmt.Errorf("无效的帧头")
	}
	
	// 读取剩余数据
	buffer := make([]byte, 4096) // 缓冲区大小
	n, err := ni.conn.Read(buffer)
	if err != nil {
		return nil, fmt.Errorf("读取数据失败: %v", err)
	}
	
	// 组合完整数据
	frameData := append(header, buffer[:n]...)
	
	// 反序列化帧
	frame, err := protocol.DeserializeTDMAFrame(frameData)
	if err != nil {
		return nil, fmt.Errorf("反序列化失败: %v", err)
	}
	
	// 验证帧
	err = frame.Validate()
	if err != nil {
		return nil, fmt.Errorf("帧验证失败: %v", err)
	}
	
	fmt.Printf("接收帧: %s\n", frame.String())
	return frame, nil
}

// 获取连接状态
func (ni *NetworkInterface) GetConnectionStatus() ConnectionStatus {
	ni.mu.RLock()
	defer ni.mu.RUnlock()
	
	return ConnectionStatus{
		Connected: ni.connected,
		Address:   ni.address,
		LastSeen:  time.Now(),
	}
}

// 设置超时
func (ni *NetworkInterface) SetTimeout(timeout time.Duration) error {
	ni.mu.Lock()
	defer ni.mu.Unlock()
	
	ni.timeout = timeout
	return nil
}

// 设置分片超时
func (ni *NetworkInterface) SetFragmentTimeout(timeout time.Duration) error {
	ni.mu.Lock()
	defer ni.mu.Unlock()
	
	ni.fragmentTimeout = timeout
	return nil
}

// 获取分片传输统计
func (ni *NetworkInterface) GetFragmentDeliveryStats() FragmentDeliveryStats {
	// 简化实现，返回默认统计
	return FragmentDeliveryStats{
		TotalFragments:     0,
		DeliveredFragments: 0,
		FailedFragments:    0,
		AverageDeliveryTime: 0,
	}
}

// 分片传输统计
type FragmentDeliveryStats struct {
	TotalFragments     int64
	DeliveredFragments int64
	FailedFragments    int64
	AverageDeliveryTime time.Duration
} 