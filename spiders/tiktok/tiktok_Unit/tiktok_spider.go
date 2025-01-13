package tiktok_Unit

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"japan_spider/pkg/cookie"
	"japan_spider/pkg/mongodb"
	"japan_spider/pkg/redis"

	"encoding/json"

	"japan_spider/spiders/tiktok/tiktok_model"

	"runtime"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// TikTokSpider 抖音爬虫结构体
type TikTokSpider struct {
	mongoClient *mongodb.MongoClient  // MongoDB客户端
	redisClient *redis.RedisClient    // Redis客户端
	cookieCtrl  *cookie.CookieControl // Cookie控制器
	config      *SpiderConfig         // 爬虫配置
	ctx         context.Context       // 添加浏览器上下文
	cancel      context.CancelFunc    // 添加取消函数
}

// SpiderConfig 爬虫配置
type SpiderConfig struct {
	ChromePath    string        // Chrome浏览器路径
	ChromeFlags   []string      // Chrome启动参数
	MongoURI      string        // MongoDB连接URI
	MongoDatabase string        // MongoDB数据库名
	RedisHost     string        // Redis主机地址
	RedisPort     int           // Redis端口
	RedisPassword string        // Redis密码
	RedisDB       int           // Redis数据库编号
	Timeout       time.Duration // 超时时间
	PythonPath    string        // Python解释器路径
	ScriptsDir    string        // Python脚本目录
}

// NewTikTokSpider 创建新的抖音爬虫实例
func NewTikTokSpider(cfg *SpiderConfig) (*TikTokSpider, error) {
	// 创建MongoDB客户端
	mongoClient, err := mongodb.NewMongoClient(&mongodb.Config{
		URI:      cfg.MongoURI,
		Database: cfg.MongoDatabase,
		Timeout:  cfg.Timeout,
	})
	if err != nil {
		return nil, fmt.Errorf("创建MongoDB客户端失败: %w", err)
	}

	// 创建Redis客户端
	redisClient, err := redis.NewRedisClient(&redis.Config{
		Host:     cfg.RedisHost,
		Port:     cfg.RedisPort,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
		Timeout:  cfg.Timeout,
	})
	if err != nil {
		return nil, fmt.Errorf("创建Redis客户端失败: %w", err)
	}

	// 创建Cookie控制器
	cookieCtrl := cookie.NewCookieControl(mongoClient, cookie.Config{
		Database:      cfg.MongoDatabase,
		Collection:    "cookies",
		MaxAge:        24 * time.Hour,
		CheckInterval: time.Hour,
	})

	return &TikTokSpider{
		mongoClient: mongoClient,
		redisClient: redisClient,
		cookieCtrl:  cookieCtrl,
		config:      cfg,
	}, nil
}

// Close 关闭爬虫，清理资源
func (s *TikTokSpider) Close() error {
	if s.cancel != nil {
		s.cancel()
	}

	var errs []error
	if err := s.mongoClient.Close(); err != nil {
		errs = append(errs, fmt.Errorf("关闭MongoDB连接失败: %w", err))
	}
	if err := s.redisClient.Close(); err != nil {
		errs = append(errs, fmt.Errorf("关闭Redis连接失败: %w", err))
	}
	if len(errs) > 0 {
		return fmt.Errorf("关闭资源时发生错误: %v", errs)
	}
	return nil
}

// Login 执行登录操作获取Cookie
func (s *TikTokSpider) Login(email, password string) error {
	// 检查Chrome路径
	if _, err := os.Stat(s.config.ChromePath); os.IsNotExist(err) {
		return fmt.Errorf("Chrome not found at path: %s", s.config.ChromePath)
	}
	log.Printf("Chrome路径验证成功: %s", s.config.ChromePath)

	// 创建浏览器选项
	var opts []chromedp.ExecAllocatorOption

	// 先尝试关闭已存在的Chrome实例
	s.killChromeProcess()
	time.Sleep(2 * time.Second)

	opts = append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.ExecPath(s.config.ChromePath),
		chromedp.Flag("remote-debugging-port", "9222"),
		chromedp.Flag("user-data-dir", filepath.Join(os.TempDir(), "chrome-data")),
		chromedp.Flag("headless", false),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-blink-features=AutomationControlled", true),
		chromedp.Flag("disable-infobars", true),
		chromedp.Flag("start-maximized", true),
		chromedp.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),
	)
	log.Printf("Chrome启动选项配置完成")

	// 创建浏览器上下文
	ctx := context.Background()
	allocCtx, cancel := chromedp.NewExecAllocator(ctx, opts...)
	var cancel2, cancel3 context.CancelFunc
	ctx, cancel2 = chromedp.NewContext(allocCtx, chromedp.WithLogf(log.Printf))
	ctx, cancel3 = context.WithTimeout(ctx, 30*time.Second)
	s.cancel = func() { cancel3(); cancel2(); cancel() }

	// 保存上下文和取消函数
	s.ctx = ctx
	s.cancel = cancel

	// 启动浏览器并检查是否成功
	log.Printf("开始启动Chrome浏览器...")
	if err := chromedp.Run(ctx); err != nil {
		s.cancel()
		return fmt.Errorf("启动Chrome失败: %w", err)
	}

	log.Println("Chrome启动成功，准备执行登录操作")
	// 等待确保Chrome完全启动
	time.Sleep(5 * time.Second)

	// 执行登录操作
	var cookies []*cookie.Cookie
	err := s.performLogin(ctx, email, password, &cookies)
	if err != nil {
		s.cancel()
		return fmt.Errorf("登录失败: %w", err)
	}

	// 获取当前IP
	ip, err := s.getCurrentIP()
	if err != nil {
		s.cancel()
		return fmt.Errorf("获取IP失败: %w", err)
	}

	// 构建用户信息
	userInfo := &tiktok_model.UserInfo{
		Email:    email,
		Password: password,
		BrowserInfo: tiktok_model.BrowserInfo{
			UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) ...",
			Version:   "120.0.0.0",
			Platform:  "Windows",
		},
		IP:           ip,
		LoginTime:    time.Now(),
		CookieValid:  true,
		LoginStatus:  true,
		LastModified: time.Now(),
		ExpireTime:   time.Now().Add(7 * 24 * time.Hour),
		Cookies:      convertCookies(cookies), // 保存Cookie数据
	}

	// 保存到Redis
	key := fmt.Sprintf("tiktok:login:%s:%s", email, ip)
	if err := s.saveToRedis(key, userInfo); err != nil {
		s.cancel()
		return fmt.Errorf("保存到Redis失败: %w", err)
	}

	// 异步保存到MongoDB
	go s.saveToMongoDB(userInfo)

	return nil
}

// 添加新方法：检查用户登录状态
func (s *TikTokSpider) CheckAndLogin(email, password string) error {
	// 生成查询key
	ip, err := s.getCurrentIP()
	if err != nil {
		return fmt.Errorf("获取IP失败: %w", err)
	}

	// 从Redis检查登录状态
	key := fmt.Sprintf("tiktok:login:%s:%s", email, ip)
	exists, err := s.redisClient.Exists(key)
	if err != nil {
		return fmt.Errorf("检查Redis登录状态失败: %w", err)
	}

	if exists {
		// 已登录，使用已有Cookie
		err = s.loginWithExistingCookie(email, ip)
		if err != nil {
			log.Printf("使用已有Cookie登录失败: %v，将执行新登录", err)
			return s.Login(email, password)
		}
		return nil
	}

	// 未登录，执行新登录
	return s.Login(email, password)
}

// 使用已存在的Cookie登录
func (s *TikTokSpider) loginWithExistingCookie(email, ip string) error {
	userInfo, err := s.getUserInfo(email, ip)
	if err != nil {
		log.Printf("获取用户信息失败，将执行新登录: %v", err)
		// 删除可能存在的无效数据
		key := fmt.Sprintf("tiktok:login:%s:%s", email, ip)
		if err := s.redisClient.RemoveKey(key); err != nil {
			log.Printf("删除无效Redis数据失败: %v", err)
		}
		// 执行新登录
		log.Printf("开始执行新登录流程...")
		return fmt.Errorf("需要重新登录")
	}

	// 检查是否过期
	if s.isUserInfoExpired(userInfo) {
		log.Printf("用户信息已过期，执行新登录")
		key := fmt.Sprintf("tiktok:login:%s:%s", email, ip)
		if err := s.redisClient.RemoveKey(key); err != nil {
			log.Printf("删除过期Redis数据失败: %v", err)
		}
		return fmt.Errorf("登录信息已过期")
	}

	// 验证Cookie是否有效
	if !userInfo.CookieValid {
		log.Printf("Cookie已失效，执行新登录")
		key := fmt.Sprintf("tiktok:login:%s:%s", email, ip)
		if err := s.redisClient.RemoveKey(key); err != nil {
			log.Printf("删除无效Redis数据失败: %v", err)
		}
		return fmt.Errorf("Cookie已失效")
	}

	// 使用Cookie打开浏览器
	return s.openBrowserWithCookie(userInfo.Cookies)
}

// saveToMongoDB 将用户信息异步保存到MongoDB
// info: 要保存的用户信息
func (s *TikTokSpider) saveToMongoDB(info *tiktok_model.UserInfo) {
	log.Printf("开始保存用户信息到MongoDB: %s", info.Email)

	// 获取集合引用
	collection := s.mongoClient.Database(s.config.MongoDatabase).Collection("tiktok_users")
	log.Printf("获取MongoDB集合: %s.tiktok_users", s.config.MongoDatabase)

	// 构建更新条件
	filter := bson.M{"email": info.Email, "ip": info.IP}
	log.Printf("构建更新条件: email=%s, ip=%s", info.Email, info.IP)

	// 执行upsert操作
	result, err := collection.UpdateOne(
		context.Background(),
		filter,
		bson.M{"$set": info},
		options.Update().SetUpsert(true),
	)

	if err != nil {
		log.Printf("保存到MongoDB失败: %v", err)
		return
	}

	// 记录操作结果
	if result.UpsertedCount > 0 {
		log.Printf("成功插入新文档: %s", info.Email)
	} else if result.ModifiedCount > 0 {
		log.Printf("成功更新现有文档: %s", info.Email)
	} else {
		log.Printf("文档未发生变化: %s", info.Email)
	}
}

// saveToRedis 将用户信息保存到Redis
// key: Redis键名
// info: 要保存的用户信息
func (s *TikTokSpider) saveToRedis(key string, info *tiktok_model.UserInfo) error {
	log.Printf("开始保存用户信息到Redis: %s", key)

	// 序列化用户信息
	data, err := json.Marshal(info)
	if err != nil {
		log.Printf("序列化用户信息失败: %v", err)
		return fmt.Errorf("序列化用户信息失败: %w", err)
	}
	log.Printf("用户信息序列化成功，数据大小: %d bytes", len(data))

	// 设置到Redis，7天过期
	expiration := 7 * 24 * time.Hour
	err = s.redisClient.SetEX(key, string(data), expiration)
	if err != nil {
		log.Printf("保存到Redis失败: %v", err)
		return fmt.Errorf("保存到Redis失败: %w", err)
	}

	log.Printf("成功保存用户信息到Redis，过期时间: %v", expiration)
	return nil
}

// 获取当前IP
func (s *TikTokSpider) getCurrentIP() (string, error) {
	output, err := s.executePythonScript("get_ip.py")
	if err != nil {
		return "", fmt.Errorf("获取IP失败: %w", err)
	}
	return strings.TrimSpace(output), nil
}

// 添加检查过期时间的方法
func (s *TikTokSpider) isUserInfoExpired(info *tiktok_model.UserInfo) bool {
	return time.Now().After(info.ExpireTime)
}

// getUserInfo 从Redis或MongoDB获取用户信息
func (s *TikTokSpider) getUserInfo(email, ip string) (*tiktok_model.UserInfo, error) {
	log.Printf("开始获取用户信息: email=%s, ip=%s", email, ip)

	// 构建Redis键名
	key := fmt.Sprintf("tiktok:login:%s:%s", email, ip)
	log.Printf("Redis键名: %s", key)

	// 先从Redis获取
	data, err := s.redisClient.Get(key)
	if err == nil && data != "" {
		log.Printf("从Redis获取到数据，开始解析")
		var userInfo tiktok_model.UserInfo
		if err := json.Unmarshal([]byte(data), &userInfo); err == nil {
			// 检查Cookie是否为空
			if len(userInfo.Cookies) > 0 {
				log.Printf("成功从Redis获取用户信息: %s", email)
				return &userInfo, nil
			}
			log.Printf("Redis中的Cookie数据为空")
		}
		log.Printf("Redis数据解析失败: %v", err)
	}
	log.Printf("Redis中未找到数据或获取失败，尝试从MongoDB获取")

	// Redis没有，从MongoDB获取
	collection := s.mongoClient.Database(s.config.MongoDatabase).Collection("tiktok_users")

	var userInfo tiktok_model.UserInfo
	err = collection.FindOne(
		context.Background(),
		bson.M{"email": email, "ip": ip},
	).Decode(&userInfo)

	if err != nil {
		log.Printf("从MongoDB获取用户信息失败: %v", err)
		return nil, fmt.Errorf("从MongoDB获取用户信息失败: %w", err)
	}

	// 将MongoDB中的数据保存到Redis
	log.Printf("从MongoDB获取数据成功，开始同步到Redis")
	if err := s.saveToRedis(key, &userInfo); err != nil {
		log.Printf("同步到Redis失败: %v", err)
		// 即使同步失败也继续使用MongoDB的数据
	} else {
		log.Printf("成功同步数据到Redis")
	}

	log.Printf("成功获取用户信息: %s (Cookie数量: %d)", email, len(userInfo.Cookies))
	return &userInfo, nil
}

// 使用Cookie打开浏览器
func (s *TikTokSpider) openBrowserWithCookie(cookies []cookie.Cookie) error {
	log.Printf("开始使用Cookie打开浏览器，Cookie数量: %d", len(cookies))

	// 连接到已运行的Chrome
	allocCtx, cancel := chromedp.NewRemoteAllocator(context.Background(), "ws://localhost:9222")
	ctx, cancel2 := chromedp.NewContext(allocCtx)
	s.ctx = ctx
	s.cancel = func() { cancel2(); cancel() }

	// 转换Cookie格式
	var networkCookies []*network.CookieParam
	for _, c := range cookies {
		log.Printf("处理Cookie: %s = %s (domain: %s)", c.Name, c.Value, c.Domain)
		if c.Domain == "" {
			c.Domain = ".tiktok.com"
		}
		networkCookies = append(networkCookies, &network.CookieParam{
			Name:     c.Name,
			Value:    c.Value,
			Domain:   c.Domain,
			Path:     c.Path,
			Secure:   c.Secure,
			HTTPOnly: c.HttpOnly,
			SameSite: "Lax",
		})
	}

	log.Printf("准备设置 %d 个Cookie", len(networkCookies))

	// 设置Cookie并打开页面
	err := chromedp.Run(ctx,
		network.Enable(),
		network.SetCookies(networkCookies),
		// 使用特定的 Tab ID 打开页面
		chromedp.Navigate("https://www.tiktok.com/foryou"),
		chromedp.Sleep(5*time.Second),
	)
	if err != nil {
		s.cancel()
		log.Printf("浏览器操作失败: %v", err)
		return fmt.Errorf("使用cookie打开浏览器失败: %w", err)
	}

	log.Printf("成功使用Cookie打开浏览器")
	return nil
}

// executePythonScript 执行Python脚本
func (s *TikTokSpider) executePythonScript(scriptName string, args ...string) (string, error) {
	scriptPath := filepath.Join(s.config.ScriptsDir, scriptName)
	log.Printf("执行Python脚本: %s", scriptPath)

	// 构建命令
	cmdArgs := append([]string{scriptPath}, args...)
	cmd := exec.Command(s.config.PythonPath, cmdArgs...)

	// 执行脚本并获取输出
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("执行Python脚本失败: %w", err)
	}

	return string(output), nil
}

// 添加这些辅助方法
func (s *TikTokSpider) checkPort(address string) error {
	conn, err := net.Dial("tcp", address)
	if err != nil {
		return err
	}
	conn.Close()
	return nil
}

func (s *TikTokSpider) killChromeProcess() {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("taskkill", "/F", "/IM", "chrome.exe")
	} else {
		cmd = exec.Command("pkill", "chrome")
	}
	cmd.Run()
	time.Sleep(2 * time.Second) // 等待进程完全退出
}
