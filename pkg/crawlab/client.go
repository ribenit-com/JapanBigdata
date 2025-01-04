package crawlab

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

type Client struct {
	BaseURL string
	ApiKey  string
}

// UploadTask 上传任务到 Crawlab
func (c *Client) UploadTask(spiderName string, data interface{}) error {
	url := fmt.Sprintf("%s/api/spiders/%s/tasks", c.BaseURL, spiderName)

	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", c.ApiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("upload failed with status: %d", resp.StatusCode)
	}

	return nil
}
