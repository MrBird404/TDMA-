#!/bin/bash

# TDMA网络协议模拟系统演示脚本

echo "=== TDMA网络协议模拟系统演示 ==="
echo ""

# 检查Go是否安装
if ! command -v go &> /dev/null; then
    echo "错误: 未找到Go编译器，请先安装Go"
    exit 1
fi

# 编译项目
echo "正在编译项目..."
go mod tidy
go build -o satellite ./cmd/satellite
go build -o groundstation ./cmd/groundstation

if [ $? -ne 0 ]; then
    echo "编译失败"
    exit 1
fi

echo "编译完成"
echo ""

# 启动卫星节点
echo "启动卫星节点..."
./satellite 8080 &
SATELLITE_PID=$!

# 等待卫星节点启动
sleep 2

# 启动地面站节点
echo "启动地面站节点..."
./groundstation GROUND_STATION_001 localhost:8080 &
GROUNDSTATION_PID=$!

echo ""
echo "系统已启动:"
echo "  卫星节点 PID: $SATELLITE_PID"
echo "  地面站节点 PID: $GROUNDSTATION_PID"
echo ""
echo "按 Ctrl+C 停止演示"

# 等待用户中断
trap "echo ''; echo '正在停止系统...'; kill $SATELLITE_PID $GROUNDSTATION_PID 2>/dev/null; rm -f satellite groundstation; echo '演示结束'; exit 0" INT

# 保持脚本运行
while true; do
    sleep 1
done 