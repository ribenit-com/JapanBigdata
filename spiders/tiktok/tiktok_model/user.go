package tiktok_model

import (
	"japan_spider/pkg/cookie"
	"time"
)

// BrowserInfo 浏览器信息
type BrowserInfo struct {
	UserAgent string `bson:"user_agent"` // 浏览器UA
	Version   string `bson:"version"`    // 浏览器版本
	Platform  string `bson:"platform"`   // 操作系统平台
}

// UserInfo 用户信息结构体
type UserInfo struct {
	Email        string          `bson:"email"`         // 用户邮箱
	Password     string          `bson:"password"`      // 密码
	BrowserInfo  BrowserInfo     `bson:"browser_info"`  // 浏览器信息
	IP           string          `bson:"ip"`            // 当前IP
	Cookies      []cookie.Cookie `bson:"cookies"`       // Cookie信息
	LoginTime    time.Time       `bson:"login_time"`    // Cookie登录时间
	CookieValid  bool            `bson:"cookie_valid"`  // Cookie是否有效
	LoginStatus  bool            `bson:"login_status"`  // 登录状态
	LastModified time.Time       `bson:"last_modified"` // 最后修改时间
	ExpireTime   time.Time       `bson:"expire_time"`   // 过期时间（7天后）
}
