package tiktok

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"japan_spider/pkg/cookie"
	"japan_spider/pkg/mongodb"
	"japan_spider/pkg/redis"
	"japan_spider/spiders/proxyPool/tiktok/model"

	"encoding/json"

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
	MongoURI      string        // MongoDB连接URI
	MongoDatabase string        // MongoDB数据库名
	RedisHost     string        // Redis主机地址
	RedisPort     int           // Redis端口
	RedisPassword string        // Redis密码
	RedisDB       int           // Redis数据库编号
	Timeout       time.Duration // 超时时间
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
	// 检查Chrome路径是否存在
	if _, err := os.Stat(s.config.ChromePath); os.IsNotExist(err) {
		log.Printf("Chrome路径不存在: %s", s.config.ChromePath)
		return fmt.Errorf("Chrome浏览器未找到: %w", err)
	}
	log.Printf("已找到Chrome浏览器: %s", s.config.ChromePath)

	// 创建浏览器选项
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.ExecPath(s.config.ChromePath),
		chromedp.Flag("headless", false),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.Flag("disable-infobars", true),
		chromedp.Flag("start-maximized", true),
		chromedp.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),
	)

	// 创建浏览器上下文
	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	ctx, cancel := chromedp.NewContext(allocCtx, chromedp.WithLogf(log.Printf))

	// 保存上下文和取消函数
	s.ctx = ctx
	s.cancel = cancel

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
	userInfo := &model.UserInfo{
		Email:    email,
		Password: password,
		BrowserInfo: model.BrowserInfo{
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
func (s *TikTokSpider) saveToMongoDB(info *model.UserInfo) {
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
func (s *TikTokSpider) saveToRedis(key string, info *model.UserInfo) error {
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
	// 实现获取当前IP的逻辑
	// 可以通过HTTP请求外部服务获取
	return "127.0.0.1", nil
}

// 添加检查过期时间的方法
func (s *TikTokSpider) isUserInfoExpired(info *model.UserInfo) bool {
	return time.Now().After(info.ExpireTime)
}

// getUserInfo 从Redis或MongoDB获取用户信息
// email: 用户邮箱
// ip: 用户IP
func (s *TikTokSpider) getUserInfo(email, ip string) (*model.UserInfo, error) {
	log.Printf("开始获取用户信息: email=%s, ip=%s", email, ip)

	// 构建Redis键名
	key := fmt.Sprintf("tiktok:login:%s:%s", email, ip)
	log.Printf("Redis键名: %s", key)

	// 先从Redis获取
	data, err := s.redisClient.Get(key)
	if err == nil && data != "" {
		log.Printf("从Redis获取到数据，开始解析")
		var userInfo model.UserInfo
		if err := json.Unmarshal([]byte(data), &userInfo); err == nil {
			// 检查Cookie是否为空
			if len(userInfo.Cookies) == 0 {
				log.Printf("用户信息中没有Cookie数据，需要重新登录")
				return nil, fmt.Errorf("Cookie数据为空")
			}
			log.Printf("成功从Redis获取用户信息: %s", email)
			return &userInfo, nil
		}
		log.Printf("Redis数据解析失败: %v", err)
	}
	log.Printf("Redis中未找到数据或获取失败: %v", err)

	// Redis没有，从MongoDB获取
	log.Printf("尝试从MongoDB获取用户信息")
	collection := s.mongoClient.Database(s.config.MongoDatabase).Collection("tiktok_users")

	var userInfo model.UserInfo
	err = collection.FindOne(
		context.Background(),
		bson.M{"email": email, "ip": ip},
	).Decode(&userInfo)

	if err != nil {
		log.Printf("从MongoDB获取用户信息失败: %v", err)
		return nil, fmt.Errorf("从MongoDB获取用户信息失败: %w", err)
	}

	log.Printf("成功从MongoDB获取用户信息: %s", email)
	return &userInfo, nil
}

// 使用Cookie打开浏览器
func (s *TikTokSpider) openBrowserWithCookie(cookies []cookie.Cookie) error {
	log.Printf("开始使用Cookie打开浏览器，Cookie数量: %d", len(cookies))

	// 创建浏览器选项
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.ExecPath(s.config.ChromePath),
		chromedp.Flag("headless", false),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.Flag("disable-infobars", true),
		chromedp.Flag("start-maximized", true),
		chromedp.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),
	)

	// 创建浏览器上下文
	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	ctx, cancel := chromedp.NewContext(allocCtx)

	// 保存上下文和取消函数
	s.ctx = ctx
	s.cancel = cancel

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
		chromedp.Navigate("https://www.tiktok.com"),
		chromedp.Sleep(5*time.Second),
	)
	if err != nil {
		s.cancel()
		log.Printf("浏览器操作失败: %v", err)
		return fmt.Errorf("使用Cookie打开浏览器失败: %w", err)
	}

	log.Printf("成功使用Cookie打开浏览器")
	return nil
}
