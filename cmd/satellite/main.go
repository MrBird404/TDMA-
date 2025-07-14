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

// 卫星节点
type SatelliteNode struct {
	nodeID    string
	scheduler *scheduler.TDMAScheduler
	network   *network.NetworkInterface
	listener  net.Listener
	running   bool
}

// 创建新的卫星节点
func NewSatelliteNode(nodeID string) *SatelliteNode {
	return &SatelliteNode{
		nodeID:    nodeID,
		scheduler: scheduler.NewTDMAScheduler(10, 1*time.Second), // 10个时隙，每个1秒
		network:   network.NewNetworkInterface(),
	}
}

// 启动卫星节点
func (sn *SatelliteNode) Start(port int) error {
	// 启动调度器
	err := sn.scheduler.Start()
	if err != nil {
		return fmt.Errorf("启动调度器失败: %v", err)
	}

	// 启动网络监听
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return fmt.Errorf("启动监听失败: %v", err)
	}
	sn.listener = listener
	sn.running = true

	fmt.Printf("卫星节点 %s 启动成功，监听端口 %d\n", sn.nodeID, port)

	// 启动接收循环
	go sn.receiveLoop()

	// 启动调度状态打印
	go sn.statusLoop()

	return nil
}

// 停止卫星节点
func (sn *SatelliteNode) Stop() error {
	sn.running = false

	if sn.listener != nil {
		sn.listener.Close()
	}

	sn.scheduler.Stop()
	sn.network.Disconnect()

	fmt.Printf("卫星节点 %s 已停止\n", sn.nodeID)
	return nil
}

// 接收循环
func (sn *SatelliteNode) receiveLoop() {
	for sn.running {
		conn, err := sn.listener.Accept()
		if err != nil {
			if sn.running {
				log.Printf("接受连接失败: %v", err)
			}
			continue
		}

		// 为每个连接启动一个处理协程
		go sn.handleConnection(conn)
	}
}

// 处理连接
func (sn *SatelliteNode) handleConnection(conn net.Conn) {
	defer conn.Close()

	log.Printf("[handleConnection] 接受来自 %s 的连接", conn.RemoteAddr())

	// 模拟网络接口连接
	sn.network.Connect(conn.RemoteAddr().String())

	buffer := make([]byte, 4096)
	for sn.running {
		// 读取帧头
		head := make([]byte, 8)
		nRead, err := conn.Read(head)
		if err != nil {
			log.Printf("[handleConnection] 读取帧头失败: %v", err)
			break
		}
		if nRead != 8 {
			log.Printf("[handleConnection] 帧头长度不足: %d", nRead)
			continue
		}
		if string(head) != string(protocol.FRAME_HEADER[:]) {
			log.Printf("[handleConnection] 无效的帧头: %v", head)
			continue
		}
		// 读取剩余部分（最大4096-8）
		n, err := conn.Read(buffer)
		if err != nil {
			log.Printf("[handleConnection] 读取帧体失败: %v", err)
			break
		}
		frameData := append(head, buffer[:n]...)
		log.Printf("[handleConnection] 收到原始数据长度: %d", len(frameData))
		// 反序列化TDMA帧
		frame, err := protocol.DeserializeTDMAFrame(frameData)
		if err != nil {
			log.Printf("[handleConnection] 解析帧失败: %v, 原始数据: %x", err, frameData)
			continue
		}
		log.Printf("[handleConnection] 成功解析帧: %s", frame.String())
		// 处理帧
		sn.processFrame(frame, conn)
	}
}

// 处理TDMA帧
func (sn *SatelliteNode) processFrame(frame *protocol.TDMAFrame, conn net.Conn) {
	log.Printf("[processFrame] 处理帧: %s", frame.String())
	// 验证帧
	err := frame.Validate()
	if err != nil {
		log.Printf("[processFrame] 帧验证失败: %v", err)
		return
	}
	// 检查是否为获取时隙请求
	if string(frame.Data) == "GET_CURRENT_SLOT" {
		currentSlot := protocol.GetGlobalSlotID(scheduler.DefaultSlotDuration, scheduler.DefaultTotalSlots)
		// 发送当前时隙响应
		respData := []byte(fmt.Sprintf("CURRENT_SLOT_%d", currentSlot))
		respFrame := protocol.NewTDMAFrame(uint32(currentSlot), sn.nodeID, respData)
		respBytes, err := respFrame.Serialize()
		if err != nil {
			log.Printf("[processFrame] 序列化响应帧失败: %v", err)
			return
		}
		_, err = conn.Write(respBytes)
		if err != nil {
			log.Printf("[processFrame] 发送响应帧失败: %v", err)
			return
		}
		log.Printf("[processFrame] 发送时隙响应: %s", respFrame.String())
		return
	}
	// 用全局统一时钟判断slotID
	currentSlot := protocol.GetGlobalSlotID(scheduler.DefaultSlotDuration, scheduler.DefaultTotalSlots)
	if int(frame.SlotID) != currentSlot {
		log.Printf("[processFrame] 时隙不匹配: 期望 %d, 实际 %d", currentSlot, frame.SlotID)
		return
	}
	nodeID := frame.GetNodeID()
	slotID, err := sn.scheduler.AllocateTimeSlot(nodeID, 1)
	if err != nil {
		log.Printf("[processFrame] 分配时隙失败: %v", err)
		return
	}
	log.Printf("[processFrame] 为节点 %s 分配时隙 %d", nodeID, slotID)
	ackData := []byte(fmt.Sprintf("ACK_SLOT_%d", slotID))
	ackFrame := protocol.NewTDMAFrame(uint32(slotID), sn.nodeID, ackData)
	ackBytes, err := ackFrame.Serialize()
	if err != nil {
		log.Printf("[processFrame] 序列化确认帧失败: %v", err)
		return
	}
	_, err = conn.Write(ackBytes)
	if err != nil {
		log.Printf("[processFrame] 发送确认帧失败: %v", err)
		return
	}
	log.Printf("[processFrame] 发送确认帧: %s", ackFrame.String())
}

// 状态循环
func (sn *SatelliteNode) statusLoop() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		sn.scheduler.PrintStatus()
	}
}

// 命令行交互
func (sn *SatelliteNode) commandLoop() {
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("卫星节点命令:")
	fmt.Println("  status - 显示状态")
	fmt.Println("  schedule - 显示调度表")
	fmt.Println("  quit - 退出")

	for sn.running {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}

		command := strings.TrimSpace(scanner.Text())

		switch command {
		case "status":
			fmt.Printf("节点ID: %s\n", sn.nodeID)
			fmt.Printf("运行状态: %v\n", sn.running)
			status := sn.network.GetConnectionStatus()
			fmt.Printf("连接状态: %v\n", status)

		case "schedule":
			schedule := sn.scheduler.GetSchedule()
			fmt.Println("当前调度表:")
			for slotID, nodeID := range schedule {
				fmt.Printf("  时隙 %d: %s\n", slotID, nodeID)
			}

		case "quit":
			sn.Stop()
			return

		default:
			fmt.Println("未知命令")
		}
	}
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("用法: satellite <端口>")
		os.Exit(1)
	}

	port, err := strconv.Atoi(os.Args[1])
	if err != nil {
		fmt.Printf("无效的端口号: %s\n", os.Args[1])
		os.Exit(1)
	}

	// 创建卫星节点
	satellite := NewSatelliteNode("SATELLITE_001")

	// 启动卫星节点
	err = satellite.Start(port)
	if err != nil {
		log.Fatalf("启动卫星节点失败: %v", err)
	}

	// 启动命令行交互
	satellite.commandLoop()
}
