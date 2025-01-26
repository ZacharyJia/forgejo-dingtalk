package config

import (
	"encoding/json"
	"fmt"
	"os"
)

// Config 表示应用配置
type Config struct {
	DingTalk struct {
		AppKey    string `json:"app_key"`
		AppSecret string `json:"app_secret"`
		AgentId   string `json:"agent_id"`
	} `json:"dingtalk"`
	SMTP struct {
		ListenAddr string `json:"listen_addr"`
		Domain     string `json:"domain"`
	} `json:"smtp"`
	UserMappings map[string]string `json:"user_mappings"`
}

// Load 从文件加载配置
func Load(filename string) (*Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %v", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %v", err)
	}

	// 验证必要的配置项
	if config.DingTalk.AppKey == "" || config.DingTalk.AppSecret == "" || config.DingTalk.AgentId == "" {
		return nil, fmt.Errorf("缺少钉钉配置")
	}
	if config.SMTP.ListenAddr == "" {
		return nil, fmt.Errorf("缺少 SMTP 监听地址配置")
	}
	if config.SMTP.Domain == "" {
		return nil, fmt.Errorf("缺少 SMTP 域名配置")
	}
	if len(config.UserMappings) == 0 {
		return nil, fmt.Errorf("缺少用户映射配置")
	}

	return &config, nil
}
