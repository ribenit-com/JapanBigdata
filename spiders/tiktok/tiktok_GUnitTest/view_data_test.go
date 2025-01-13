// Package main 提供TikTok爬虫的测试功能
package main

import (
	"log"
	"testing"
)

// TestViewData 测试查看用户数据的功能
// 显示Redis和MongoDB中存储的用户登录信息
func TestViewData(t *testing.T) {
	// 创建数据查看器实例
	viewer, err := NewDataViewer()
	if err != nil {
		// 如果创建失败，终止测试
		t.Fatalf("创建数据查看器失败: %v", err)
	}
	// 确保资源在测试结束时被释放
	defer viewer.Close()

	// 设置要查询的用户邮箱
	email := "zuandilong@gmail.com"

	// 查看Redis中存储的数据
	log.Println("=== 查看Redis数据 ===")
	if err := viewer.ViewRedisLoginInfo(email); err != nil {
		// 如果查询失败，记录错误但继续测试
		t.Errorf("查看Redis数据失败: %v", err)
	}

	// 查看MongoDB中存储的数据
	log.Println("\n=== 查看MongoDB数据 ===")
	if err := viewer.ViewMongoLoginInfo(email); err != nil {
		// 如果查询失败，记录错误但继续测试
		t.Errorf("查看MongoDB数据失败: %v", err)
	}
}

// TestInvalidateLogin 测试使登录状态失效的功能
// 包括查看原始状态、执行失效操作和确认更新结果
func TestInvalidateLogin(t *testing.T) {
	// 创建数据查看器实例
	viewer, err := NewDataViewer()
	if err != nil {
		// 如果创建失败，终止测试
		t.Fatalf("创建数据查看器失败: %v", err)
	}
	// 确保资源在测试结束时被释放
	defer viewer.Close()

	// 设置要操作的用户邮箱
	email := "zuandilong@gmail.com"

	// 首先查看用户当前的登录状态
	log.Println("=== 当前登录状态 ===")
	if err := viewer.ViewRedisLoginInfo(email); err != nil {
		// 如果查询失败，记录错误但继续测试
		t.Errorf("查看Redis数据失败: %v", err)
	}
	if err := viewer.ViewMongoLoginInfo(email); err != nil {
		// 如果查询失败，记录错误但继续测试
		t.Errorf("查看MongoDB数据失败: %v", err)
	}

	// 执行登录状态失效操作
	log.Println("\n=== 使登录状态失效 ===")
	if err := viewer.InvalidateLogin(email); err != nil {
		// 如果操作失败，记录错误但继续测试
		t.Errorf("使登录状态失效失败: %v", err)
	}

	// 查看更新后的状态以确认更改是否生效
	log.Println("\n=== 确认状态更新 ===")
	if err := viewer.ViewRedisLoginInfo(email); err != nil {
		// 如果查询失败，记录错误但继续测试
		t.Errorf("查看Redis数据失败: %v", err)
	}
	if err := viewer.ViewMongoLoginInfo(email); err != nil {
		// 如果查询失败，记录错误但继续测试
		t.Errorf("查看MongoDB数据失败: %v", err)
	}
}
