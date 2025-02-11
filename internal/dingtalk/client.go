package dingtalk

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Client 钉钉应用客户端
type Client struct {
	appKey    string
	appSecret string
	agentId   string
	baseURL   string
	client    *http.Client

	tokenMu    sync.RWMutex
	token      string
	expireTime time.Time
}

// TokenResponse 访问令牌响应
type TokenResponse struct {
	ErrCode   int    `json:"errcode"`
	ErrMsg    string `json:"errmsg"`
	Token     string `json:"access_token"`
	ExpiresIn int    `json:"expires_in"`
}

// MessageResponse 发送消息响应
type MessageResponse struct {
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
	TaskId  int64  `json:"task_id"`
}

// NewClient 创建新的钉钉应用客户端
func NewClient(appKey, appSecret, agentId string) *Client {
	return &Client{
		appKey:    appKey,
		appSecret: appSecret,
		agentId:   agentId,
		baseURL:   "https://oapi.dingtalk.com",
		client:    &http.Client{},
	}
}

// GetToken 获取访问令牌
func (c *Client) GetToken() (string, error) {
	c.tokenMu.RLock()
	if c.token != "" && time.Now().Before(c.expireTime) {
		token := c.token
		c.tokenMu.RUnlock()
		return token, nil
	}
	c.tokenMu.RUnlock()

	c.tokenMu.Lock()
	defer c.tokenMu.Unlock()

	// 双重检查，避免并发刷新token
	if c.token != "" && time.Now().Before(c.expireTime) {
		return c.token, nil
	}

	url := fmt.Sprintf("%s/gettoken?appkey=%s&appsecret=%s", c.baseURL, c.appKey, c.appSecret)
	resp, err := c.client.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to get token: %v", err)
	}
	defer resp.Body.Close()

	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("failed to decode token response: %v", err)
	}

	if tokenResp.ErrCode != 0 {
		return "", fmt.Errorf("failed to get token: %s", tokenResp.ErrMsg)
	}

	c.token = tokenResp.Token
	c.expireTime = time.Now().Add(time.Duration(tokenResp.ExpiresIn-60) * time.Second)

	return c.token, nil
}

// SendMessage 发送工作通知消息
func (c *Client) SendMessage(userIDs string, content string) error {
	token, err := c.GetToken()
	if err != nil {
		return err
	}

	// 将用户ID字符串分割成切片
	userIDList := splitUserIDs(userIDs)
	
	// 按照每批100个用户进行分组发送
	batchSize := 100
	for i := 0; i < len(userIDList); i += batchSize {
		end := i + batchSize
		if end > len(userIDList) {
			end = len(userIDList)
		}
		
		// 将当前批次的用户ID合并成逗号分隔的字符串
		batchUserIDs := joinUserIDs(userIDList[i:end])
		
		url := fmt.Sprintf("%s/topapi/message/corpconversation/asyncsend_v2?access_token=%s", c.baseURL, token)

		agentID, _ := strconv.Atoi(c.agentId)
		requestBody := map[string]interface{}{
			"agent_id":    agentID,
			"userid_list": batchUserIDs,
			"msg": map[string]interface{}{
				"msgtype": "markdown",
				"markdown": map[string]string{
					"title": "Git系统通知",
					"text":  content,
				},
			},
		}

		jsonBody, err := json.Marshal(requestBody)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %v", err)
		}

		resp, err := c.client.Post(url, "application/json", bytes.NewBuffer(jsonBody))
		if err != nil {
			return fmt.Errorf("failed to send message: %v", err)
		}
		defer resp.Body.Close()

		var msgResp MessageResponse
		if err := json.NewDecoder(resp.Body).Decode(&msgResp); err != nil {
			return fmt.Errorf("failed to decode message response: %v", err)
		}

		if msgResp.ErrCode != 0 {
			return fmt.Errorf("failed to send message: %s", msgResp.ErrMsg)
		}
	}

	return nil
}

// splitUserIDs 将逗号分隔的用户ID字符串分割成切片
func splitUserIDs(userIDs string) []string {
	if userIDs == "" {
		return nil
	}
	return strings.Split(userIDs, ",")
}

// joinUserIDs 将用户ID切片合并成逗号分隔的字符串
func joinUserIDs(userIDs []string) string {
	return strings.Join(userIDs, ",")
}

// GetUserIdByMobile 通过手机号获取用户ID
func (c *Client) GetUserIdByMobile(mobile string) (string, error) {
	token, err := c.GetToken()
	if err != nil {
		return "", err
	}

	url := fmt.Sprintf("%s/topapi/v2/user/getbymobile?access_token=%s", c.baseURL, token)
	requestBody := map[string]string{
		"mobile": mobile,
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request body: %v", err)
	}

	resp, err := c.client.Post(url, "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to get user: %v", err)
	}
	defer resp.Body.Close()

	var result struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
		Result  struct {
			UserId string `json:"userid"`
		} `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %v", err)
	}

	if result.ErrCode != 0 {
		return "", fmt.Errorf("failed to get user: %s", result.ErrMsg)
	}

	return result.Result.UserId, nil
}
