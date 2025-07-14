package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"tdma-network/internal/network"
	"tdma-network/internal/scheduler"
	"tdma-network/pkg/protocol"
	"time"
)

// 地面站节点
type GroundStationNode struct {
	nodeID  string
	network *network.NetworkInterface
	conn    net.Conn
	running bool
	slotID  int // 固定分配的slotID
}

// 创建新的地面站节点
func NewGroundStationNode(nodeID string) *GroundStationNode {
	return &GroundStationNode{
		nodeID:  nodeID,
		network: network.NewNetworkInterface(),
		slotID:  -1,
	}
}

// 连接到卫星节点
func (gsn *GroundStationNode) ConnectToSatellite(address string) error {
	conn, err := net.Dial("tcp", address)
	if err != nil {
		return fmt.Errorf("连接卫星节点失败: %v", err)
	}

	gsn.conn = conn
	gsn.running = true

	fmt.Printf("地面站节点 %s 已连接到卫星节点 %s\n", gsn.nodeID, address)

	// 启动接收循环
	go gsn.receiveLoop()

	return nil
}

// 断开连接
func (gsn *GroundStationNode) Disconnect() error {
	gsn.running = false

	if gsn.conn != nil {
		gsn.conn.Close()
		gsn.conn = nil
	}

	fmt.Printf("地面站节点 %s 已断开连接\n", gsn.nodeID)
	return nil
}

// 发送TDMA帧
func (gsn *GroundStationNode) SendFrame(slotID int, data []byte) error {
	if gsn.conn == nil {
		return fmt.Errorf("未连接到卫星节点")
	}

	// 创建TDMA帧
	frame := protocol.NewTDMAFrame(uint32(slotID), gsn.nodeID, data)

	// 序列化帧
	frameBytes, err := frame.Serialize()
	if err != nil {
		return fmt.Errorf("序列化帧失败: %v", err)
	}

	// 发送帧
	_, err = gsn.conn.Write(frameBytes)
	if err != nil {
		return fmt.Errorf("发送帧失败: %v", err)
	}

	fmt.Printf("发送帧: %s\n", frame.String())
	return nil
}

// 获取卫星当前时隙
func (gsn *GroundStationNode) GetCurrentSlot() (int, error) {
	if gsn.conn == nil {
		log.Printf("[GetCurrentSlot] 未连接到卫星节点")
		return -1, fmt.Errorf("未连接到卫星节点")
	}
	log.Printf("[GetCurrentSlot] 发送GET_CURRENT_SLOT请求")
	frame := protocol.NewTDMAFrame(0, gsn.nodeID, []byte("GET_CURRENT_SLOT"))
	frameBytes, err := frame.Serialize()
	if err != nil {
		log.Printf("[GetCurrentSlot] 序列化帧失败: %v", err)
		return -1, fmt.Errorf("序列化帧失败: %v", err)
	}
	_, err = gsn.conn.Write(frameBytes)
	if err != nil {
		log.Printf("[GetCurrentSlot] 发送请求失败: %v", err)
		return -1, fmt.Errorf("发送请求失败: %v", err)
	}
	buffer := make([]byte, 1024)
	gsn.conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, err := gsn.conn.Read(buffer)
	if err != nil {
		log.Printf("[GetCurrentSlot] 读取响应失败: %v", err)
		return -1, fmt.Errorf("读取响应失败: %v", err)
	}
	respFrame, err := protocol.DeserializeTDMAFrame(buffer[:n])
	if err != nil {
		log.Printf("[GetCurrentSlot] 解析响应帧失败: %v", err)
		return -1, fmt.Errorf("解析响应帧失败: %v", err)
	}
	slotStr := string(respFrame.Data)
	log.Printf("[GetCurrentSlot] 收到响应: %s", slotStr)
	if !strings.HasPrefix(slotStr, "CURRENT_SLOT_") {
		log.Printf("[GetCurrentSlot] 无效的响应格式: %s", slotStr)
		return -1, fmt.Errorf("无效的响应格式")
	}
	slotStr = strings.TrimPrefix(slotStr, "CURRENT_SLOT_")
	slot, err := strconv.Atoi(slotStr)
	if err != nil {
		log.Printf("[GetCurrentSlot] 解析时隙失败: %v", err)
		return -1, fmt.Errorf("解析时隙失败: %v", err)
	}
	log.Printf("[GetCurrentSlot] 获取到卫星当前时隙: %d", slot)
	return slot, nil
}

// 发送默认数据
func (gsn *GroundStationNode) SendDefaultData() error {
	log.Printf("[SendDefaultData] 开始发送默认数据流程")
	// 获取当前全局slotID
	slotDuration := scheduler.DefaultSlotDuration
	totalSlots := scheduler.DefaultTotalSlots
	currentSlot := protocol.GetGlobalSlotID(slotDuration, totalSlots)
	log.Printf("[SendDefaultData] 当前全局slotID: %d, 我的固定slotID: %d", currentSlot, gsn.slotID)
	if currentSlot != gsn.slotID {
		log.Printf("[SendDefaultData] 当前不是我的时隙，跳过发送")
		return nil
	}
	defaultData := []byte(fmt.Sprintf("DEFAULT_DATA_FROM_%s_%d", gsn.nodeID, time.Now().Unix()))
	log.Printf("[SendDefaultData] 使用时隙: %d, 数据: %s", gsn.slotID, string(defaultData))
	err := gsn.SendFrame(gsn.slotID, defaultData)
	if err != nil {
		log.Printf("[SendDefaultData] 发送帧失败: %v", err)
		return err
	}
	log.Printf("[SendDefaultData] 发送帧成功")
	return nil
}

// 接收循环
func (gsn *GroundStationNode) receiveLoop() {
	for gsn.running {
		if gsn.conn == nil {
			time.Sleep(100 * time.Millisecond)
			continue
		}

		// 设置读取超时
		gsn.conn.SetReadDeadline(time.Now().Add(5 * time.Second))

		// 读取数据
		buffer := make([]byte, 1024)
		n, err := gsn.conn.Read(buffer)
		if err != nil {
			if gsn.running {
				log.Printf("读取数据失败: %v", err)
			}
			continue
		}

		// 解析TDMA帧
		frame, err := protocol.DeserializeTDMAFrame(buffer[:n])
		if err != nil {
			log.Printf("解析帧失败: %v", err)
			continue
		}

		// 跳过控制帧（如CURRENT_SLOT响应）
		if strings.HasPrefix(string(frame.Data), "CURRENT_SLOT_") {
			continue
		}

		// 处理帧
		gsn.processFrame(frame)
	}
}

// 处理接收到的帧
func (gsn *GroundStationNode) processFrame(frame *protocol.TDMAFrame) {
	fmt.Printf("接收帧: %s\n", frame.String())

	// 验证帧
	err := frame.Validate()
	if err != nil {
		log.Printf("帧验证失败: %v", err)
		return
	}

	// 检查是否为确认帧
	if strings.Contains(string(frame.Data), "ACK_SLOT") {
		// 解析分配的时隙
		ackData := string(frame.Data)
		if strings.HasPrefix(ackData, "ACK_SLOT_") {
			slotStr := strings.TrimPrefix(ackData, "ACK_SLOT_")
			slotID, err := strconv.Atoi(slotStr)
			if err == nil {
				gsn.slotID = slotID
				fmt.Printf("收到时隙分配确认: 时隙 %d\n", slotID)
			}
		}
	} else {
		// 处理其他类型的帧
		fmt.Printf("收到数据: %s\n", string(frame.Data))
	}
}

// 自动发送循环
func (gsn *GroundStationNode) autoSendLoop() {
	log.Printf("[autoSendLoop] 自动发送循环启动")
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		log.Printf("[autoSendLoop] 定时触发发送默认数据")
		err := gsn.SendDefaultData()
		if err != nil {
			log.Printf("[autoSendLoop] 发送默认数据失败: %v", err)
		} else {
			log.Printf("[autoSendLoop] 发送默认数据成功")
		}
	}
}

// 命令行交互
func (gsn *GroundStationNode) commandLoop() {
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("地面站节点命令:")
	fmt.Println("  send - 发送默认数据")
	fmt.Println("  status - 显示状态")
	fmt.Println("  quit - 退出")

	for gsn.running {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}

		command := strings.TrimSpace(scanner.Text())

		switch command {
		case "send":
			err := gsn.SendDefaultData()
			if err != nil {
				fmt.Printf("发送失败: %v\n", err)
			} else {
				fmt.Println("发送成功")
			}

		case "status":
			fmt.Printf("节点ID: %s\n", gsn.nodeID)
			fmt.Printf("运行状态: %v\n", gsn.running)
			fmt.Printf("当前时隙: %d\n", gsn.slotID)
			if gsn.conn != nil {
				fmt.Printf("连接状态: 已连接\n")
			} else {
				fmt.Printf("连接状态: 未连接\n")
			}

		case "quit":
			gsn.Disconnect()
			return

		default:
			fmt.Println("未知命令")
		}
	}
}

func main() {
	log.Printf("[main] 地面站节点启动，参数: %v", os.Args)
	if len(os.Args) < 4 {
		fmt.Println("用法: groundstation <节点ID> <卫星地址:端口> <slotID>")
		os.Exit(1)
	}

	nodeID := os.Args[1]
	satelliteAddress := os.Args[2]
	slotID, err := strconv.Atoi(os.Args[3])
	if err != nil {
		fmt.Printf("slotID参数无效: %v\n", err)
		os.Exit(1)
	}

	// 创建地面站节点
	groundStation := NewGroundStationNode(nodeID)
	groundStation.slotID = slotID
	log.Printf("[main] 创建地面站节点: %s, 固定slotID: %d", nodeID, slotID)

	// 连接到卫星节点
	err = groundStation.ConnectToSatellite(satelliteAddress)
	if err != nil {
		log.Fatalf("[main] 连接卫星节点失败: %v", err)
	}
	log.Printf("[main] 已连接到卫星节点: %s", satelliteAddress)

	// 启动自动发送循环
	go groundStation.autoSendLoop()
	log.Printf("[main] 自动发送循环已启动")

	// 启动命令行交互
	groundStation.commandLoop()
}
