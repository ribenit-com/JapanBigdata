package tiktok

import (
	"context"
	"fmt"
	"log"
	"time"

	"japan_spider/pkg/cookie"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

// performLogin 执行登录操作
func (s *TikTokSpider) performLogin(ctx context.Context, email, password string, cookies *[]*cookie.Cookie) error {
	// 执行登录流程
	err := chromedp.Run(ctx,
		// 导航到登录页面
		s.navigateToLogin(),

		// 等待并填写登录表单
		s.fillLoginForm(email, password),

		// 等待手动验证
		s.waitForManualVerification(),

		// 获取Cookies
		s.getCookies(cookies),
	)

	if err != nil {
		return fmt.Errorf("登录操作失败: %w", err)
	}

	return nil
}

// navigateToLogin 导航到登录页面
func (s *TikTokSpider) navigateToLogin() chromedp.Tasks {
	return chromedp.Tasks{
		chromedp.ActionFunc(func(ctx context.Context) error {
			log.Println("开始导航到登录页面...")
			return nil
		}),
		chromedp.Navigate("https://www.tiktok.com/login/phone-or-email/email"),
		chromedp.Sleep(5 * time.Second),
	}
}

// fillLoginForm 填写登录表单
func (s *TikTokSpider) fillLoginForm(email, password string) chromedp.Tasks {
	return chromedp.Tasks{
		// 等待登录表单出现
		chromedp.ActionFunc(func(ctx context.Context) error {
			log.Println("等待登录表单出现...")
			return nil
		}),
		chromedp.WaitVisible(`//input[@type="text"]`, chromedp.BySearch),

		// 输入邮箱和密码
		chromedp.SendKeys(`//input[@type="text"]`, email, chromedp.BySearch),
		chromedp.SendKeys(`//input[@type="password"]`, password, chromedp.BySearch),

		// 点击登录按钮
		chromedp.Click(`//button[contains(text(), "登录")]`, chromedp.BySearch),
		chromedp.Sleep(5 * time.Second),
	}
}

// waitForManualVerification 等待手动验证
func (s *TikTokSpider) waitForManualVerification() chromedp.Tasks {
	return chromedp.Tasks{
		chromedp.ActionFunc(func(ctx context.Context) error {
			log.Println("请手动完成验证...")
			return nil
		}),
		chromedp.Sleep(30 * time.Second),
	}
}

// getCookies 获取Cookies
func (s *TikTokSpider) getCookies(cookies *[]*cookie.Cookie) chromedp.Tasks {
	return chromedp.Tasks{
		chromedp.ActionFunc(func(ctx context.Context) error {
			log.Println("开始获取Cookies...")
			netCookies, err := network.GetCookies().Do(ctx)
			if err != nil {
				return fmt.Errorf("获取Cookies失败: %w", err)
			}

			if len(netCookies) == 0 {
				return fmt.Errorf("没有获取到任何Cookie")
			}

			*cookies = convertNetworkCookies(netCookies)
			log.Printf("成功获取 %d 个Cookies", len(*cookies))
			return nil
		}),
	}
}
