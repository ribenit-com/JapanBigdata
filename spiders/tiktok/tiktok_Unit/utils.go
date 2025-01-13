package tiktok_Unit

import (
	"japan_spider/pkg/cookie"
	"time"

	"github.com/chromedp/cdproto/network"
)

// 辅助函数：转换网络Cookie到自定义Cookie结构
func convertNetworkCookies(netCookies []*network.Cookie) []*cookie.Cookie {
	var cookies []*cookie.Cookie
	for _, c := range netCookies {
		cookie := &cookie.Cookie{
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
func convertCookies(cookies []*cookie.Cookie) []cookie.Cookie {
	result := make([]cookie.Cookie, len(cookies))
	for i, c := range cookies {
		result[i] = *c
	}
	return result
}
