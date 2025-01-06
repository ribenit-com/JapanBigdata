package cookie

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"japan_spider/pkg/mongodb"

	"go.mongodb.org/mongo-driver/mongo/options"
)

// Cookie 结构体定义单个Cookie的信息
type Cookie struct {
	Name       string    `json:"name" bson:"name"`               // Cookie名称
	Value      string    `json:"value" bson:"value"`             // Cookie值
	Domain     string    `json:"domain" bson:"domain"`           // 所属域名
	Path       string    `json:"path" bson:"path"`               // Cookie路径
	Expires    time.Time `json:"expires" bson:"expires"`         // 过期时间
	Secure     bool      `json:"secure" bson:"secure"`           // 是否只通过HTTPS传输
	HttpOnly   bool      `json:"http_only" bson:"http_only"`     // 是否只允许HTTP访问
	CreateTime time.Time `json:"create_time" bson:"create_time"` // 创建时间
	LastUsed   time.Time `json:"last_used" bson:"last_used"`     // 最后使用时间
}

// Session 会话结构体，管理一组Cookie
type Session struct {
	ID        string    `json:"id" bson:"_id"`          // 会话ID
	UserID    string    `json:"user_id" bson:"user_id"` // 用户ID
	Cookies   []Cookie  `json:"cookies" bson:"cookies"` // Cookie列表
	CreatedAt time.Time `json:"created_at" bson:"created_at"`
	UpdatedAt time.Time `json:"updated_at" bson:"updated_at"`
	ExpiresAt time.Time `json:"expires_at" bson:"expires_at"`
}

// CookieControl Cookie管理器
type CookieControl struct {
	sessions      map[string]*Session  // 会话存储
	mu            sync.RWMutex         // 读写锁
	mongoClient   *mongodb.MongoClient // MongoDB客户端
	database      string               // 数据库名
	collection    string               // 集合名
	maxAge        time.Duration        // Cookie最大存活时间
	checkInterval time.Duration        // 过期检查间隔
}

// NewCookieControl 创建新的Cookie控制器
func NewCookieControl(mongoClient *mongodb.MongoClient, config Config) *CookieControl {
	cc := &CookieControl{
		sessions:      make(map[string]*Session),
		mongoClient:   mongoClient,
		database:      config.Database,
		collection:    config.Collection,
		maxAge:        config.MaxAge,
		checkInterval: config.CheckInterval,
	}

	// 启动定期清理
	go cc.startCleanup()

	return cc
}

// SaveSession 保存会话到MongoDB
func (cc *CookieControl) SaveSession(session *Session) error {
	coll := cc.mongoClient.Client().Database(cc.database).Collection(cc.collection)

	session.UpdatedAt = time.Now()
	_, err := coll.UpdateOne(
		cc.mongoClient.Context(),
		map[string]interface{}{"_id": session.ID},
		map[string]interface{}{"$set": session},
		options.Update().SetUpsert(true),
	)

	return err
}

// GetSession 获取会话信息
func (cc *CookieControl) GetSession(sessionID string) (*Session, error) {
	cc.mu.RLock()
	if session, exists := cc.sessions[sessionID]; exists {
		cc.mu.RUnlock()
		return session, nil
	}
	cc.mu.RUnlock()

	// 从MongoDB获取
	coll := cc.mongoClient.Client().Database(cc.database).Collection(cc.collection)
	var session Session
	err := coll.FindOne(cc.mongoClient.Context(), map[string]interface{}{"_id": sessionID}).Decode(&session)
	if err != nil {
		return nil, err
	}

	// 缓存到内存
	cc.mu.Lock()
	cc.sessions[sessionID] = &session
	cc.mu.Unlock()

	return &session, nil
}

// UpdateCookies 更新会话的Cookie
func (cc *CookieControl) UpdateCookies(sessionID string, cookies []Cookie) error {
	session, err := cc.GetSession(sessionID)
	if err != nil {
		return err
	}

	session.Cookies = cookies
	session.UpdatedAt = time.Now()

	return cc.SaveSession(session)
}

// DeleteExpiredSessions 删除过期会话
func (cc *CookieControl) DeleteExpiredSessions() error {
	now := time.Now()
	coll := cc.mongoClient.Client().Database(cc.database).Collection(cc.collection)

	_, err := coll.DeleteMany(
		cc.mongoClient.Context(),
		map[string]interface{}{
			"expires_at": map[string]interface{}{"$lt": now},
		},
	)

	return err
}

// startCleanup 启动定期清理过期会话
func (cc *CookieControl) startCleanup() {
	ticker := time.NewTicker(cc.checkInterval)
	for range ticker.C {
		if err := cc.DeleteExpiredSessions(); err != nil {
			log.Printf("清理过期会话失败: %v", err)
		}
	}
}

// ExportCookies 导出会话的Cookie为JSON
func (cc *CookieControl) ExportCookies(sessionID string) (string, error) {
	session, err := cc.GetSession(sessionID)
	if err != nil {
		return "", err
	}

	data, err := json.MarshalIndent(session.Cookies, "", "  ")
	if err != nil {
		return "", err
	}

	return string(data), nil
}

// ImportCookies 从JSON导入Cookie到会话
func (cc *CookieControl) ImportCookies(sessionID string, jsonData string) error {
	var cookies []Cookie
	if err := json.Unmarshal([]byte(jsonData), &cookies); err != nil {
		return err
	}

	return cc.UpdateCookies(sessionID, cookies)
}

// GetValidCookies 获取会话中的有效Cookie
func (cc *CookieControl) GetValidCookies(sessionID string) ([]Cookie, error) {
	session, err := cc.GetSession(sessionID)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	validCookies := make([]Cookie, 0)

	for _, cookie := range session.Cookies {
		if cookie.Expires.After(now) {
			validCookies = append(validCookies, cookie)
		}
	}

	return validCookies, nil
}
