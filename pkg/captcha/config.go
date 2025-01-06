package captcha

import "time"

// Config 验证码控制器配置
type Config struct {
	Database           string        // MongoDB数据库名
	AllowManual        bool          // 是否允许人工介入
	ManualTimeout      time.Duration // 人工验证超时时间
	CacheCleanInterval time.Duration // 缓存清理间隔
	CacheTTL           time.Duration // 缓存有效期
}

// ManualRequest 人工验证请求
type ManualRequest struct {
	ID        string    `json:"id" bson:"_id"`            // 请求ID
	Type      string    `json:"type" bson:"type"`         // 验证码类型
	ImageData string    `json:"image" bson:"image"`       // Base64图片数据
	Solution  string    `json:"solution" bson:"solution"` // 解决方案
	CreatedAt time.Time `json:"created_at" bson:"created_at"`
	SolvedAt  time.Time `json:"solved_at" bson:"solved_at"`
}
