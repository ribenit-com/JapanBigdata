package main

import (
	"log"
	"testing"
)

func TestViewData(t *testing.T) {
	// 创建数据查看器
	viewer, err := NewDataViewer()
	if err != nil {
		t.Fatalf("创建数据查看器失败: %v", err)
	}
	defer viewer.Close()

	email := "zuandilong@gmail.com"

	// 查看Redis中的数据
	log.Println("=== 查看Redis数据 ===")
	if err := viewer.ViewRedisLoginInfo(email); err != nil {
		t.Errorf("查看Redis数据失败: %v", err)
	}

	// 查看MongoDB中的数据
	log.Println("\n=== 查看MongoDB数据 ===")
	if err := viewer.ViewMongoLoginInfo(email); err != nil {
		t.Errorf("查看MongoDB数据失败: %v", err)
	}
}

// TestInvalidateLogin 测试使登录状态失效
func TestInvalidateLogin(t *testing.T) {
	// 创建数据查看器
	viewer, err := NewDataViewer()
	if err != nil {
		t.Fatalf("创建数据查看器失败: %v", err)
	}
	defer viewer.Close()

	email := "zuandilong@gmail.com"

	// 查看原始状态
	log.Println("=== 当前登录状态 ===")
	if err := viewer.ViewRedisLoginInfo(email); err != nil {
		t.Errorf("查看Redis数据失败: %v", err)
	}
	if err := viewer.ViewMongoLoginInfo(email); err != nil {
		t.Errorf("查看MongoDB数据失败: %v", err)
	}

	// 使登录状态失效
	log.Println("\n=== 使登录状态失效 ===")
	if err := viewer.InvalidateLogin(email); err != nil {
		t.Errorf("使登录状态失效失败: %v", err)
	}

	// 确认状态已更新
	log.Println("\n=== 确认状态更新 ===")
	if err := viewer.ViewRedisLoginInfo(email); err != nil {
		t.Errorf("查看Redis数据失败: %v", err)
	}
	if err := viewer.ViewMongoLoginInfo(email); err != nil {
		t.Errorf("查看MongoDB数据失败: %v", err)
	}
}
