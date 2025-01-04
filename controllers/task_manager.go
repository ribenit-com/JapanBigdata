// controllers/task_manager.go
package controllers

import (
	"context"
	"fmt"
	"sync"
)

// TaskManager 管理爬虫任务的生命周期，支持任务的启动和取消
// tasks: 存储任务 ID 和对应的取消函数
// tasksMux: 读写锁，保护任务的并发安全访问
// activeTasks: 用于统计当前正在运行的任务数量
// maxTasks: 限制同时运行的任务数量，避免资源超载

type TaskManager struct {
	tasks       map[string]context.CancelFunc // 存储任务 ID 和其取消函数
	tasksMux    sync.RWMutex                  // 保护任务操作的互斥锁
	activeTasks int                           // 当前正在运行的任务数量
	maxTasks    int                           // 最大允许的任务数量
}

// NewTaskManager 创建一个新的 TaskManager 实例
func NewTaskManager(maxTasks int) *TaskManager {
	return &TaskManager{
		tasks:    make(map[string]context.CancelFunc),
		maxTasks: maxTasks,
	}
}

// StartTask 启动一个新的任务
// taskId: 任务的唯一标识符
// task: 任务逻辑，接受一个上下文对象
func (tm *TaskManager) StartTask(taskId string, task func(context.Context)) error {
	tm.tasksMux.Lock()
	defer tm.tasksMux.Unlock()

	// 检查是否超过最大任务限制
	if tm.activeTasks >= tm.maxTasks {
		return fmt.Errorf("任务启动失败: 超过最大任务限制 %d", tm.maxTasks)
	}

	// 创建带取消功能的上下文
	ctx, cancel := context.WithCancel(context.Background())
	tm.tasks[taskId] = cancel
	tm.activeTasks++

	// 启动任务，运行完成后移除任务
	go func() {
		task(ctx)
		tm.completeTask(taskId)
	}()

	return nil
}

// CancelTask 取消指定任务
// taskId: 任务的唯一标识符
func (tm *TaskManager) CancelTask(taskId string) error {
	tm.tasksMux.Lock()
	defer tm.tasksMux.Unlock()

	cancel, exists := tm.tasks[taskId]
	if !exists {
		return fmt.Errorf("任务取消失败: 找不到任务 %s", taskId)
	}

	cancel()
	tm.removeTask(taskId)
	return nil
}

// completeTask 处理任务完成的清理逻辑
// taskId: 任务的唯一标识符
func (tm *TaskManager) completeTask(taskId string) {
	tm.tasksMux.Lock()
	delete(tm.tasks, taskId)
	tm.activeTasks--
	tm.tasksMux.Unlock()
}

// removeTask 从任务映射中移除指定任务
// taskId: 任务的唯一标识符
func (tm *TaskManager) removeTask(taskId string) {
	delete(tm.tasks, taskId)
}
