// controllers/node_manager.go
package controllers

import (
	"fmt"
	"sync"
)

// Node 表示 Crawlab 系统中的节点信息
// 包括节点的 ID、名称和状态

type Node struct {
	ID     string // 节点唯一标识符
	Name   string // 节点名称
	Status string // 节点状态 (idle, busy, offline)
}

// NodeManager 用于管理 Crawlab 的节点
// 提供添加节点、删除节点和查询节点状态等功能

type NodeManager struct {
	nodes    map[string]*Node // 存储节点信息的映射
	nodesMux sync.RWMutex     // 保护节点操作的互斥锁
}

// NewNodeManager 创建并初始化一个新的 NodeManager
func NewNodeManager() *NodeManager {
	return &NodeManager{
		nodes: make(map[string]*Node),
	}
}

// AddNode 添加一个新的节点到管理器
// id: 节点的唯一标识符
// name: 节点名称
// status: 节点初始状态
func (nm *NodeManager) AddNode(id, name, status string) {
	nm.nodesMux.Lock()
	defer nm.nodesMux.Unlock()

	if _, exists := nm.nodes[id]; exists {
		fmt.Printf("节点已存在: %s\n", id)
		return
	}

	nm.nodes[id] = &Node{
		ID:     id,
		Name:   name,
		Status: status,
	}

	fmt.Printf("节点添加成功: %s (%s)\n", name, id)
}

// RemoveNode 从管理器中移除指定节点
// id: 节点的唯一标识符
func (nm *NodeManager) RemoveNode(id string) {
	nm.nodesMux.Lock()
	defer nm.nodesMux.Unlock()

	if _, exists := nm.nodes[id]; !exists {
		fmt.Printf("节点不存在: %s\n", id)
		return
	}

	delete(nm.nodes, id)
	fmt.Printf("节点移除成功: %s\n", id)
}

// UpdateNodeStatus 更新指定节点的状态
// id: 节点的唯一标识符
// status: 节点的新状态
func (nm *NodeManager) UpdateNodeStatus(id, status string) {
	nm.nodesMux.Lock()
	defer nm.nodesMux.Unlock()

	node, exists := nm.nodes[id]
	if !exists {
		fmt.Printf("节点不存在: %s\n", id)
		return
	}

	node.Status = status
	fmt.Printf("节点状态更新成功: %s -> %s\n", id, status)
}

// GetNodeStatus 查询指定节点的状态
// id: 节点的唯一标识符
// 返回节点状态和是否存在的标志
func (nm *NodeManager) GetNodeStatus(id string) (string, bool) {
	nm.nodesMux.RLock()
	defer nm.nodesMux.RUnlock()

	node, exists := nm.nodes[id]
	if !exists {
		return "", false
	}

	return node.Status, true
}

// ListNodes 列出所有节点的信息
func (nm *NodeManager) ListNodes() []Node {
	nm.nodesMux.RLock()
	defer nm.nodesMux.RUnlock()

	nodeList := make([]Node, 0, len(nm.nodes))
	for _, node := range nm.nodes {
		nodeList = append(nodeList, *node)
	}

	return nodeList
}
