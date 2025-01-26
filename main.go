package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/zacharyjia/forgejo-dingtalk/internal/config"
	"github.com/zacharyjia/forgejo-dingtalk/internal/dingtalk"
	"github.com/zacharyjia/forgejo-dingtalk/internal/smtp"
)

var (
	configPath = flag.String("config", "config.json", "Path to config file")
)

func main() {
	flag.Parse()

	// 加载配置
	cfg, err := config.Load(*configPath)
	if err != nil {
		// 如果配置文件不存在，创建示例配置文件
		if os.IsNotExist(err) {
			log.Printf("配置文件 %s 不存在，请参考 config.example.json 创建配置文件", *configPath)
			os.Exit(1)
		}
		log.Fatal("加载配置文件失败:", err)
	}

	// 创建钉钉客户端
	dingTalkClient := dingtalk.NewClient(
		cfg.DingTalk.AppKey,
		cfg.DingTalk.AppSecret,
		cfg.DingTalk.AgentId,
	)

	// 创建并启动SMTP服务器
	smtpServer := smtp.NewServer(cfg, dingTalkClient)

	// 创建一个用于接收关闭信号的通道
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// 在goroutine中启动SMTP服务器
	go func() {
		if err := smtpServer.Start(); err != nil {
			log.Printf("SMTP服务器错误: %v", err)
		}
	}()

	// 等待关闭信号
	<-stop
	log.Println("正在关闭服务器...")

	// 创建一个带超时的context用于优雅关闭
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 关闭SMTP服务器
	if err := smtpServer.Stop(); err != nil {
		log.Printf("关闭SMTP服务器时发生错误: %v", err)
	}

	// 等待context超时或取消
	<-ctx.Done()
	log.Println("服务器已关闭")
}
