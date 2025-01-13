package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"japan_spider/spiders/tiktok"
)

func main() {
	// 设置日志格式
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	log.Printf("启动TikTok爬虫...")

	// 创建配置
	config := &tiktok.SpiderConfig{
		ChromePath:    `C:\Users\Administrator\AppData\Local\Google\Chrome\Bin\Google Chrome.exe`, // 实际Chrome路径
		MongoURI:      "mongodb://192.168.20.6:30643",
		MongoDatabase: "spider",
		RedisHost:     "192.168.20.6",
		RedisPort:     32430,
		RedisPassword: "",
		RedisDB:       0,
		Timeout:       5 * time.Minute,
		PythonPath:    "python",     // 或具体的Python路径
		ScriptsDir:    "../scripts", // 修正脚本目录路径
	}

	// 创建爬虫实例
	spider, err := tiktok.NewTikTokSpider(config)
	if err != nil {
		log.Fatalf("创建爬虫失败: %v", err)
	}
	defer spider.Close()

	// 检查并登录
	log.Printf("开始登录流程...")
	err = spider.CheckAndLogin("zuandilong@gmail.com", "Jia@hong565")
	if err != nil {
		log.Fatalf("登录失败: %v", err)
	}

	log.Println("----------------------------------------")
	log.Println("✅ 所有自动化流程已完成!")
	log.Println("🔍 当前状态:")
	log.Println("   - MongoDB数据已保存")
	log.Println("   - Redis缓存已更新")
	log.Println("   - 浏览器会话已建立")
	log.Println("----------------------------------------")
	log.Println("💡 提示: 程序将保持运行以维持会话")
	log.Println("⌨️  按 Ctrl+C 可以随时退出程序")
	log.Println("----------------------------------------")

	// 等待中断信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("\n收到退出信号，正在清理资源...")
	log.Println("程序已完全退出，运行结束。")
}
