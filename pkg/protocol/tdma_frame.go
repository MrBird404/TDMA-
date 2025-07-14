package protocol

import (
	"encoding/binary"
	"fmt"
	"strings"
	"time"
)

// TDMA帧结构 (扩展版)
type TDMAFrame struct {
	Header     [8]byte
	SlotID     uint32
	NodeID     [32]byte // 扩大为32字节
	Length     uint32
	FragmentID uint32 // 分片ID
	TotalFrags uint16 // 总分片数
	FragIndex  uint16 // 当前分片索引
	Flags      uint16 // 分片标志位
	Data       []byte
	CRC        uint32
	Footer     [8]byte
}

// 分片标志位定义
const (
	FLAG_FRAGMENT   = 0x0001 // 是否为分片
	FLAG_FIRST_FRAG = 0x0002 // 是否为第一个分片
	FLAG_LAST_FRAG  = 0x0004 // 是否为最后一个分片
	FLAG_NEED_ACK   = 0x0008 // 是否需要确认
)

// 帧头帧尾常量
var (
	FRAME_HEADER = [8]byte{0xAA, 0x55, 0xAA, 0x55, 0xAA, 0x55, 0xAA, 0x55}
	FRAME_FOOTER = [8]byte{0x55, 0xAA, 0x55, 0xAA, 0x55, 0xAA, 0x55, 0xAA}
)

// 全局TDMA时隙起点
var TDMA_EPOCH = time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

// 获取全局统一时钟下的slotID
func GetGlobalSlotID(slotDuration time.Duration, totalSlots int) int {
	now := time.Now().UTC()
	elapsed := now.Sub(TDMA_EPOCH)
	slot := int(elapsed/slotDuration) % totalSlots
	return slot
}

// 创建新的TDMA帧
func NewTDMAFrame(slotID uint32, nodeID string, data []byte) *TDMAFrame {
	frame := &TDMAFrame{
		Header:     FRAME_HEADER,
		SlotID:     slotID,
		Length:     uint32(len(data)),
		FragmentID: 0, // 非分片
		TotalFrags: 1,
		FragIndex:  0,
		Flags:      0, // 非分片
		Data:       data,
		Footer:     FRAME_FOOTER,
	}

	// 设置节点ID
	copy(frame.NodeID[:], []byte(nodeID))

	// 计算CRC
	frame.CRC = calculateCRC(frame)

	return frame
}

// 创建分片TDMA帧
func NewFragmentTDMAFrame(slotID uint32, nodeID string, fragmentID uint32,
	totalFrags uint16, fragIndex uint16, data []byte, isFirst, isLast bool) *TDMAFrame {

	frame := &TDMAFrame{
		Header:     FRAME_HEADER,
		SlotID:     slotID,
		Length:     uint32(len(data)),
		FragmentID: fragmentID,
		TotalFrags: totalFrags,
		FragIndex:  fragIndex,
		Flags:      FLAG_FRAGMENT,
		Data:       data,
		Footer:     FRAME_FOOTER,
	}

	// 设置分片标志
	if isFirst {
		frame.Flags |= FLAG_FIRST_FRAG
	}
	if isLast {
		frame.Flags |= FLAG_LAST_FRAG
	}

	// 设置节点ID
	copy(frame.NodeID[:], []byte(nodeID))

	// 计算CRC
	frame.CRC = calculateCRC(frame)

	return frame
}

// 序列化TDMA帧
func (f *TDMAFrame) Serialize() ([]byte, error) {
	// 计算总长度
	totalLen := 8 + 4 + 32 + 4 + 4 + 2 + 2 + 2 + len(f.Data) + 4 + 8

	buf := make([]byte, totalLen)
	offset := 0

	// 写入Header
	copy(buf[offset:], f.Header[:])
	offset += 8

	// 写入SlotID
	binary.BigEndian.PutUint32(buf[offset:], f.SlotID)
	offset += 4

	// 写入NodeID
	copy(buf[offset:], f.NodeID[:])
	offset += 32

	// 写入Length
	binary.BigEndian.PutUint32(buf[offset:], f.Length)
	offset += 4

	// 写入FragmentID
	binary.BigEndian.PutUint32(buf[offset:], f.FragmentID)
	offset += 4

	// 写入TotalFrags
	binary.BigEndian.PutUint16(buf[offset:], f.TotalFrags)
	offset += 2

	// 写入FragIndex
	binary.BigEndian.PutUint16(buf[offset:], f.FragIndex)
	offset += 2

	// 写入Flags
	binary.BigEndian.PutUint16(buf[offset:], f.Flags)
	offset += 2

	// 写入Data
	copy(buf[offset:], f.Data)
	offset += len(f.Data)

	// 写入CRC
	binary.BigEndian.PutUint32(buf[offset:], f.CRC)
	offset += 4

	// 写入Footer
	copy(buf[offset:], f.Footer[:])

	return buf, nil
}

// 反序列化TDMA帧
func DeserializeTDMAFrame(data []byte) (*TDMAFrame, error) {
	if len(data) < 60 { // 最小长度变为60
		return nil, fmt.Errorf("数据长度不足")
	}

	frame := &TDMAFrame{}
	offset := 0

	// 读取Header
	copy(frame.Header[:], data[offset:offset+8])
	offset += 8

	// 读取SlotID
	frame.SlotID = binary.BigEndian.Uint32(data[offset:])
	offset += 4

	// 读取NodeID
	copy(frame.NodeID[:], data[offset:offset+32])
	offset += 32

	// 读取Length
	frame.Length = binary.BigEndian.Uint32(data[offset:])
	offset += 4

	// 读取FragmentID
	frame.FragmentID = binary.BigEndian.Uint32(data[offset:])
	offset += 4

	// 读取TotalFrags
	frame.TotalFrags = binary.BigEndian.Uint16(data[offset:])
	offset += 2

	// 读取FragIndex
	frame.FragIndex = binary.BigEndian.Uint16(data[offset:])
	offset += 2

	// 读取Flags
	frame.Flags = binary.BigEndian.Uint16(data[offset:])
	offset += 2

	// 读取Data
	dataLen := int(frame.Length)
	if offset+dataLen+12 > len(data) {
		return nil, fmt.Errorf("数据长度不匹配")
	}
	frame.Data = make([]byte, dataLen)
	copy(frame.Data, data[offset:offset+dataLen])
	offset += dataLen

	// 读取CRC
	frame.CRC = binary.BigEndian.Uint32(data[offset:])
	offset += 4

	// 读取Footer
	copy(frame.Footer[:], data[offset:offset+8])

	return frame, nil
}

// 验证帧完整性
func (f *TDMAFrame) Validate() error {
	// 检查帧头
	if f.Header != FRAME_HEADER {
		return fmt.Errorf("无效的帧头")
	}

	// 检查帧尾
	if f.Footer != FRAME_FOOTER {
		return fmt.Errorf("无效的帧尾")
	}

	// 检查CRC
	calculatedCRC := calculateCRC(f)
	if calculatedCRC != f.CRC {
		return fmt.Errorf("CRC校验失败")
	}

	// 检查数据长度
	if uint32(len(f.Data)) != f.Length {
		return fmt.Errorf("数据长度不匹配")
	}

	return nil
}

// 检查是否为分片
func (f *TDMAFrame) IsFragment() bool {
	return (f.Flags & FLAG_FRAGMENT) != 0
}

// 检查是否为第一个分片
func (f *TDMAFrame) IsFirstFragment() bool {
	return (f.Flags & FLAG_FIRST_FRAG) != 0
}

// 检查是否为最后一个分片
func (f *TDMAFrame) IsLastFragment() bool {
	return (f.Flags & FLAG_LAST_FRAG) != 0
}

// 获取节点ID字符串
func (f *TDMAFrame) GetNodeID() string {
	return strings.TrimRight(string(f.NodeID[:]), "\x00")
}

// 计算CRC
func calculateCRC(frame *TDMAFrame) uint32 {
	// 简化的CRC计算，实际应用中应使用更复杂的算法
	var crc uint32 = 0xFFFFFFFF

	// 计算Header的CRC
	for _, b := range frame.Header {
		crc = (crc << 1) ^ uint32(b)
	}

	// 计算其他字段的CRC
	crc = (crc << 1) ^ frame.SlotID
	crc = (crc << 1) ^ uint32(frame.TotalFrags)
	crc = (crc << 1) ^ uint32(frame.FragIndex)
	crc = (crc << 1) ^ uint32(frame.Flags)

	// 计算数据的CRC
	for _, b := range frame.Data {
		crc = (crc << 1) ^ uint32(b)
	}

	return crc
}

// 格式化输出帧信息
func (f *TDMAFrame) String() string {
	return fmt.Sprintf("TDMAFrame{SlotID:%d, NodeID:%s, Length:%d, FragmentID:%d, TotalFrags:%d, FragIndex:%d, Flags:0x%04X, DataLen:%d}",
		f.SlotID, f.GetNodeID(), f.Length, f.FragmentID, f.TotalFrags, f.FragIndex, f.Flags, len(f.Data))
}
