package proxy

import (
	"testing"
	"time"

	"japan_spider/pkg/mongodb"
	"japan_spider/pkg/redis"
)

// 测试辅助函数：清理Redis测试数据
func cleanupRedis(t *testing.T, client *redis.RedisClient) {
	const redisKey = "current_proxy_batch"
	if err := client.RemoveKey(redisKey); err != nil {
		t.Logf("清理Redis数据失败: %v", err)
	}
}

// 测试辅助函数：准备测试环境
func setupTest(t *testing.T) (*ProxyPool, *mongodb.MongoClient, *redis.RedisClient) {
	pool := NewProxyPool(Config{
		BatchSize: 500,
		Timeout:   5 * time.Second,
	})

	mongoClient := createTestMongoClient(t)
	redisClient := createTestRedisClient(t)

	return pool, mongoClient, redisClient
}

// 测试辅助函数：清理测试环境
func teardownTest(t *testing.T, mongoClient *mongodb.MongoClient, redisClient *redis.RedisClient) {
	cleanupRedis(t, redisClient)
	mongoClient.Close()
	redisClient.Close()
}

// 创建测试用的MongoDB客户端
func createTestMongoClient(t *testing.T) *mongodb.MongoClient {
	cfg := &mongodb.Config{
		URI:      "mongodb://192.168.20.6:30643",
		Database: "proxy_pool_test",
		Timeout:  5 * time.Second,
	}
	client, err := mongodb.NewMongoClient(cfg)
	if err != nil {
		t.Fatalf("创建MongoDB客户端失败: %v", err)
	}
	return client
}

// 创建测试用的Redis客户端
func createTestRedisClient(t *testing.T) *redis.RedisClient {
	cfg := &redis.Config{
		Host:     "192.168.20.6",
		Port:     32430,
		Password: "",
		DB:       1, // 使用不同的数据库避免影响生产环境
		Timeout:  5 * time.Second,
	}
	client, err := redis.NewRedisClient(cfg)
	if err != nil {
		t.Fatalf("创建Redis客户端失败: %v", err)
	}
	return client
}

// 测试创建代理池
func TestNewProxyPool(t *testing.T) {
	config := Config{
		BatchSize: 500,
		Timeout:   5 * time.Second,
	}
	pool := NewProxyPool(config)
	if pool == nil {
		t.Fatal("创建代理池失败")
	}
	if pool.proxies == nil {
		t.Error("代理列表未初始化")
	}
}

// 测试添加代理
func TestAddProxy(t *testing.T) {
	pool := NewProxyPool(Config{Timeout: 5 * time.Second})

	tests := []struct {
		name     string
		url      string
		protocol string
		wantErr  bool
	}{
		{
			name:     "有效代理",
			url:      "http://127.0.0.1:8080",
			protocol: "http",
			wantErr:  false,
		},
		{
			name:     "无效URL",
			url:      "invalid-url",
			protocol: "http",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := pool.AddProxy(tt.url, tt.protocol)
			if (err != nil) != tt.wantErr {
				t.Errorf("AddProxy() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// 测试从MongoDB加载代理到Redis
func TestLoadProxiesFromMongo(t *testing.T) {
	pool, mongoClient, redisClient := setupTest(t)
	defer teardownTest(t, mongoClient, redisClient)

	err := pool.LoadProxiesFromMongo(mongoClient, redisClient, 10)
	if err != nil {
		t.Errorf("LoadProxiesFromMongo() error = %v", err)
	}
}

// 测试获取下一个可用代理
func TestGetNextValidProxy(t *testing.T) {
	pool, mongoClient, redisClient := setupTest(t)
	defer teardownTest(t, mongoClient, redisClient)

	proxy, err := pool.GetNextValidProxy(redisClient, mongoClient)
	if err != nil {
		t.Errorf("GetNextValidProxy() error = %v", err)
	}
	if proxy != nil {
		if proxy.URL == "" {
			t.Error("代理URL为空")
		}
		if proxy.Protocol == "" {
			t.Error("代理协议为空")
		}
		if !proxy.Available {
			t.Error("代理不可用")
		}
	}
}

// 测试刷新代理池
func TestRefreshProxyPool(t *testing.T) {
	pool, mongoClient, redisClient := setupTest(t)
	defer teardownTest(t, mongoClient, redisClient)

	err := pool.RefreshProxyPool(redisClient, mongoClient, 5)
	if err != nil {
		t.Errorf("RefreshProxyPool() error = %v", err)
	}
}

// 测试移除代理
func TestRemoveProxy(t *testing.T) {
	pool := NewProxyPool(Config{Timeout: 5 * time.Second})

	// 添加测试代理
	testURL := "http://127.0.0.1:8080"
	pool.AddProxy(testURL, "http")

	// 确保代理被添加
	if len(pool.proxies) != 1 {
		t.Fatal("代理添加失败")
	}

	// 移除代理
	pool.RemoveProxy(testURL)

	// 确保代理被移除
	if len(pool.proxies) != 0 {
		t.Error("代理移除失败")
	}
}
