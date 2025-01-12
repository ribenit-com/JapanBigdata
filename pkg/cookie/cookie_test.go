package cookie

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
	"testing"
	"time"

	"japan_spider/pkg/mongodb"
	"japan_spider/pkg/redis"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

// 测试配置
type testConfig struct {
	mongoClient *mongodb.MongoClient
	redisClient *redis.RedisClient
	cookieCtrl  *CookieControl
}

// 设置测试环境
func setupTest(t *testing.T) *testConfig {
	// 创建MongoDB客户端
	mongoClient, err := mongodb.NewMongoClient(&mongodb.Config{
		URI:      "mongodb://192.168.20.6:30643",
		Database: "spider",
		Timeout:  10 * time.Second,
	})
	if err != nil {
		t.Fatalf("创建MongoDB客户端失败: %v", err)
	}

	// 创建Redis客户端
	redisClient, err := redis.NewRedisClient(&redis.Config{
		Host:     "192.168.20.6",
		Port:     32430,
		Password: "",
		DB:       0,
		Timeout:  10 * time.Second,
	})
	if err != nil {
		t.Fatalf("创建Redis客户端失败: %v", err)
	}

	// 创建Cookie控制器
	cc := NewCookieControl(mongoClient, Config{
		Database:      "spider",
		Collection:    "cookies",
		MaxAge:        24 * time.Hour,
		CheckInterval: time.Hour,
	})

	return &testConfig{
		mongoClient: mongoClient,
		redisClient: redisClient,
		cookieCtrl:  cc,
	}
}

// 清理测试环境
func teardownTest(t *testing.T, cfg *testConfig) {
	if err := cfg.mongoClient.Close(); err != nil {
		t.Errorf("关闭MongoDB连接失败: %v", err)
	}
	if err := cfg.redisClient.Close(); err != nil {
		t.Errorf("关闭Redis连接失败: %v", err)
	}
}

// TestMongoDBConnection 测试MongoDB连接
func TestMongoDBConnection(t *testing.T) {
	cfg := setupTest(t)
	defer teardownTest(t, cfg)

	// 测试MongoDB连接
	if err := cfg.mongoClient.Ping(); err != nil {
		t.Errorf("MongoDB连接测试失败: %v", err)
	}
}

// TestRedisConnection 测试Redis连接
func TestRedisConnection(t *testing.T) {
	cfg := setupTest(t)
	defer teardownTest(t, cfg)

	// 测试Redis连接
	if err := cfg.redisClient.Ping(); err != nil {
		t.Errorf("Redis连接测试失败: %v", err)
	}
}

// TestTikTokLogin 测试TikTok登录获取Cookie
func TestTikTokLogin(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过需要长时间运行的测试")
	}

	log.Println("开始TikTok登录测试...")
	cfg := setupTest(t)
	t.Cleanup(func() {
		log.Println("清理测试环境...")
		teardownTest(t, cfg)
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	t.Cleanup(func() {
		cancel()
	})

	errChan := make(chan error, 1)
	doneChan := make(chan bool, 1)

	go func() {
		defer close(errChan)
		defer close(doneChan)

		log.Println("开始执行登录流程...")
		cookies, err := performTikTokLogin(t)
		if err != nil {
			errChan <- fmt.Errorf("TikTok登录失败: %v", err)
			return
		}
		log.Printf("登录成功，获取到 %d 个cookies", len(cookies))

		log.Println("开始保存Cookies...")
		if err := saveCookies(t, cfg, cookies); err != nil {
			errChan <- fmt.Errorf("保存Cookie失败: %v", err)
			return
		}
		log.Println("Cookies保存成功")

		log.Println("开始验证Cookies...")
		if err := verifyCookies(t, cfg); err != nil {
			errChan <- fmt.Errorf("验证Cookie失败: %v", err)
			return
		}
		log.Println("Cookies验证成功")

		doneChan <- true
	}()

	select {
	case err := <-errChan:
		if err != nil {
			t.Fatal(err)
		}
	case <-doneChan:
		log.Println("测试成功完成")
	case <-ctx.Done():
		t.Fatal("测试超时")
	}
}

// performTikTokLogin 执行TikTok登录流程
func performTikTokLogin(t *testing.T) ([]*Cookie, error) {
	var chromePath string
	switch runtime.GOOS {
	case "windows":
		chromePath = `C:\Users\Administrator\AppData\Local\Google\Chrome\Bin\chrome.exe`
	}

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.ExecPath(chromePath),
		chromedp.Flag("headless", false),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.Flag("disable-infobars", true),
		chromedp.Flag("start-maximized", true),
		chromedp.Flag("no-first-run", true),
		chromedp.Flag("no-default-browser-check", true),
		chromedp.Flag("ignore-certificate-errors", true),
		chromedp.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),
	)

	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()

	ctx, cancel := chromedp.NewContext(allocCtx,
		chromedp.WithLogf(log.Printf), // 启用日志
	)
	defer cancel()

	// 增加超时时间到5分钟
	ctx, cancel = context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	var cookies []*Cookie

	// 修改错误处理部分
	err := chromedp.Run(ctx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			log.Println("开始导航到登录页面...")
			return nil
		}),
		chromedp.Navigate("https://www.tiktok.com/login/phone-or-email/email"),
		chromedp.Sleep(5*time.Second),

		// 等待页面加载完成
		chromedp.ActionFunc(func(ctx context.Context) error {
			var title string
			if err := chromedp.Title(&title).Do(ctx); err == nil {
				log.Printf("页面标题: %s", title)
			}
			return nil
		}),

		// 等待登录表单出现
		chromedp.ActionFunc(func(ctx context.Context) error {
			log.Println("等待登录表单出现...")
			selectors := []string{
				`//input[@type="text"]`,                    // 邮箱输入框
				`//input[@type="password"]`,                // 密码输入框
				`//button[contains(text(), "登录")]`,         // 登录按钮
				`//div[contains(text(), "使用手机/邮箱/用户名登录")]`, // 登录文本
			}

			for _, selector := range selectors {
				log.Printf("尝试选择器: %s", selector)
				var nodes []*cdp.Node
				if err := chromedp.Nodes(selector, &nodes, chromedp.BySearch).Do(ctx); err == nil {
					log.Printf("找到 %d 个匹配元素: %s", len(nodes), selector)
					if len(nodes) > 0 {
						return nil
					}
				}
			}
			return fmt.Errorf("未找到登录表单")
		}),

		// 直接输入邮箱和密码
		chromedp.SendKeys(`//input[@type="text"]`, "zuandilong@gmail.com", chromedp.BySearch),
		chromedp.SendKeys(`//input[@type="password"]`, "Jia@hong565", chromedp.BySearch),

		// 点击登录按钮
		chromedp.Click(`//button[contains(text(), "登录")]`, chromedp.BySearch),
		chromedp.Sleep(5*time.Second),

		// 截图查看页面状态
		chromedp.ActionFunc(func(ctx context.Context) error {
			var buf []byte
			if err := chromedp.FullScreenshot(&buf, 90).Do(ctx); err != nil {
				return fmt.Errorf("截图失败: %w", err)
			}
			// 保存截图
			if err := os.WriteFile("login_page.png", buf, 0644); err != nil {
				return fmt.Errorf("保存截图失败: %w", err)
			}
			log.Println("页面截图已保存为 login_page.png")
			return nil
		}),

		// 获取页面HTML
		chromedp.ActionFunc(func(ctx context.Context) error {
			var html string
			if err := chromedp.OuterHTML("html", &html).Do(ctx); err != nil {
				return fmt.Errorf("获取HTML失败: %w", err)
			}
			// 保存HTML
			if err := os.WriteFile("login_page.html", []byte(html), 0644); err != nil {
				return fmt.Errorf("保存HTML失败: %w", err)
			}
			log.Println("页面HTML已保存为 login_page.html")
			return nil
		}),

		// 尝试多个可能的登录容器选择器
		chromedp.ActionFunc(func(ctx context.Context) error {
			log.Println("等待登录容器出现...")

			// 先获取并打印页面标题
			var title string
			if err := chromedp.Title(&title).Do(ctx); err == nil {
				log.Printf("页面标题: %s", title)
			}

			// 检查页面是否有特定元素
			selectors := []string{
				// 更通用的选择器
				`//div[contains(@class, "tiktok")]`,
				`//div[contains(@class, "jsx")]`,
				`//div[contains(@class, "container")]`,
				// 表单相关选择器
				`//form`,
				`//input`,
				`//button`,
				// 登录相关文本
				`//*[contains(text(), "Log in")]`,
				`//*[contains(text(), "Sign in")]`,
				`//*[contains(text(), "Continue")]`,
			}

			for _, selector := range selectors {
				log.Printf("尝试选择器: %s", selector)
				var nodes []*cdp.Node
				if err := chromedp.Nodes(selector, &nodes, chromedp.BySearch).Do(ctx); err == nil {
					log.Printf("找到 %d 个匹配元素: %s", len(nodes), selector)
					if len(nodes) > 0 {
						return nil
					}
				}
			}

			// 如果没有找到任何元素，获取body内容
			var body string
			if err := chromedp.InnerHTML("body", &body).Do(ctx); err == nil {
				log.Printf("页面内容长度: %d", len(body))
				// 保存body内容以供分析
				if err := os.WriteFile("page_body.txt", []byte(body), 0644); err == nil {
					log.Println("页面内容已保存到 page_body.txt")
				}
			}

			return fmt.Errorf("未找到登录容器")
		}),
		chromedp.Sleep(2*time.Second),

		// 在旋转盘验证前暂停
		chromedp.ActionFunc(func(ctx context.Context) error {
			log.Println("请手动完成旋转盘验证...")
			return nil
		}),

		chromedp.Sleep(30*time.Second),

		// 等待登录成功
		chromedp.ActionFunc(func(ctx context.Context) error {
			log.Println("等待登录完成...")
			return nil
		}),

		// 获取Cookies
		chromedp.ActionFunc(func(ctx context.Context) error {
			log.Println("开始获取Cookies...")
			cookies2, err := network.GetCookies().Do(ctx)
			if err != nil {
				return fmt.Errorf("获取Cookies失败: %w", err)
			}
			if len(cookies2) == 0 {
				return fmt.Errorf("没有获取到任何Cookie")
			}
			cookies = convertNetworkCookies(cookies2)
			log.Printf("成功获取 %d 个Cookies", len(cookies))
			return nil
		}),
	)

	if err != nil {
		log.Printf("登录操作失败: %v", err)
		return nil, fmt.Errorf("登录操作失败: %w", err)
	}

	log.Printf("成功获取 %d 个Cookies", len(cookies))
	for _, cookie := range cookies {
		log.Printf("Cookie详情: Name=%s, Domain=%s, Expires=%v",
			cookie.Name, cookie.Domain, cookie.Expires)
	}

	return cookies, nil
}

// saveCookies 保存Cookie到MongoDB和Redis
func saveCookies(t *testing.T, cfg *testConfig, cookies []*Cookie) error {
	// 创建会话
	session := &Session{
		ID:        "tiktok_test_session",
		UserID:    "zuandilong@gmail.com",
		Cookies:   convertCookies(cookies),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	// 保存到MongoDB
	if err := cfg.cookieCtrl.SaveSession(session); err != nil {
		return err
	}

	// 保存到Redis
	cookieKey := "tiktok:cookies:zuandilong@gmail.com"
	for _, cookie := range cookies {
		if cookie.Name == "sessionid" || cookie.Name == "tt_csrf_token" {
			if err := cfg.redisClient.HSet(cookieKey, cookie.Name, cookie.Value); err != nil {
				return err
			}
		}
	}

	return cfg.redisClient.Expire(cookieKey, 24*time.Hour)
}

// verifyCookies 验证保存的Cookie
func verifyCookies(t *testing.T, cfg *testConfig) error {
	// 验证MongoDB中的Cookie
	savedSession, err := cfg.cookieCtrl.GetSession("tiktok_test_session")
	if err != nil {
		return err
	}

	if len(savedSession.Cookies) == 0 {
		t.Error("保存的Cookie为空")
	}

	// 验证Redis中的Cookie
	cookieKey := "tiktok:cookies:zuandilong@gmail.com"
	sessionid, err := cfg.redisClient.HGet(cookieKey, "sessionid")
	if err != nil {
		return err
	}

	if sessionid == "" {
		t.Error("Redis中的sessionid为空")
	}

	return nil
}

// 辅助函数：转换网络Cookie到自定义Cookie结构
func convertNetworkCookies(netCookies []*network.Cookie) []*Cookie {
	var cookies []*Cookie
	for _, c := range netCookies {
		cookie := &Cookie{
			Name:       c.Name,
			Value:      c.Value,
			Domain:     c.Domain,
			Path:       c.Path,
			Expires:    time.Unix(int64(c.Expires), 0),
			Secure:     c.Secure,
			HttpOnly:   c.HTTPOnly,
			CreateTime: time.Now(),
			LastUsed:   time.Now(),
		}
		cookies = append(cookies, cookie)
	}
	return cookies
}

// 辅助函数：转换Cookie指针切片到Cookie值切片
func convertCookies(cookies []*Cookie) []Cookie {
	result := make([]Cookie, len(cookies))
	for i, c := range cookies {
		result[i] = *c
	}
	return result
}

// TestVerifyLoginStatus 验证登录状态
func TestVerifyLoginStatus(t *testing.T) {
	cfg := setupTest(t)
	defer teardownTest(t, cfg)

	// 确保清理旧的Chrome进程
	cleanup := func() {
		switch runtime.GOOS {
		case "windows":
			exec.Command("taskkill", "/F", "/IM", "chrome.exe").Run()
			exec.Command("taskkill", "/F", "/IM", "chromedriver.exe").Run()
		default:
			exec.Command("pkill", "chrome").Run()
			exec.Command("pkill", "chromedriver").Run()
		}
		time.Sleep(2 * time.Second)
	}

	cleanup()
	defer cleanup()

	// 创建新的浏览器上下文
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	var chromePath string
	switch runtime.GOOS {
	case "windows":
		chromePath = `C:\Users\Administrator\AppData\Local\Google\Chrome\Bin\chrome.exe`
	}

	// 创建新的浏览器实例
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.ExecPath(chromePath),
		chromedp.Flag("headless", false),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.Flag("disable-infobars", true),
		chromedp.Flag("start-maximized", true),
		chromedp.Flag("no-first-run", true),
		chromedp.Flag("no-default-browser-check", true),
		chromedp.Flag("ignore-certificate-errors", true),
		chromedp.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),
	)

	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()

	// 创建新的浏览器实例
	ctx, cancel = chromedp.NewContext(allocCtx)
	defer cancel()

	// 设置超时
	ctx, cancel = context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	// 加载保存的Cookies
	cookieKey := "tiktok:cookies:zuandilong@gmail.com"
	sessionid, err := cfg.redisClient.HGet(cookieKey, "sessionid")
	if err != nil || sessionid == "" {
		t.Fatal("无法加载保存的sessionid")
	}

	// 创建cookie参数
	cookies := []*network.CookieParam{
		{
			Name:     "sessionid",
			Value:    sessionid,
			Domain:   ".tiktok.com",
			Path:     "/",
			Secure:   true,
			HTTPOnly: true,
		},
	}

	// 先导航到目标域名，然后设置cookie
	err = chromedp.Run(ctx,
		chromedp.Navigate("https://www.tiktok.com"),
		network.SetCookies(cookies),
		chromedp.Reload(),
		chromedp.Sleep(5*time.Second),
	)
	if err != nil {
		t.Fatalf("设置Cookies失败: %v", err)
	}

	// 检查是否需要重新登录
	var loginText string
	err = chromedp.Run(ctx,
		chromedp.Text(`//button[contains(text(), "登录")]`, &loginText, chromedp.BySearch),
	)
	if err == nil && loginText != "" {
		t.Fatal("需要重新登录")
	} else {
		log.Println("已登录，无需重新登录")
	}
}
