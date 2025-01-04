package crawlab

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

// Client Crawlab API客户端
// 用于与Crawlab平台进行通信
type Client struct {
	BaseURL string // Crawlab服务器地址
	ApiKey  string // API认证密钥
}

// UploadTask 将爬取的数据上传到Crawlab平台
// spiderName: 爬虫名称，用于在Crawlab中标识数据来源
// data: 要上传的数据，将被转换为JSON格式
func (c *Client) UploadTask(spiderName string, data interface{}) error {
	// 构造API请求URL
	url := fmt.Sprintf("%s/api/spiders/%s/tasks", c.BaseURL, spiderName)

	// 将数据转换为JSON格式
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	// 创建HTTP请求
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}

	// 设置请求头
	req.Header.Set("Authorization", c.ApiKey)
	req.Header.Set("Content-Type", "application/json")

	// 发送请求
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// 检查响应状态
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("upload failed with status: %d", resp.StatusCode)
	}

	return nil
}
