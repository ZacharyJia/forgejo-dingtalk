# Forgejo DingTalk 通知服务

该服务通过 SMTP 服务器接收 Forgejo (Gitea) 的邮件通知，并将其转发到钉钉企业内部应用，实现代码库变更的实时通知。

## 功能特点

- 基于 SMTP 协议接收 Forgejo 邮件通知
- 使用钉钉企业内部应用进行私聊通知
- 支持邮箱到钉钉用户的映射配置
- 完全兼容 Forgejo 的邮件通知系统
- 自动提取邮件主题和内容
- 支持优雅关闭

## 使用前提

1. 创建钉钉企业内部应用
   - 登录[钉钉开放平台](https://open.dingtalk.com/)
   - 创建企业内部应用
   - 获取应用的 AppKey 和 AppSecret
   - 设置应用权限：
     - 通讯录管理权限：以手机号获取用户 ID
     - 消息通知权限：发送工作通知

2. 配置 Forgejo 邮件设置
   - 在 Forgejo 管理后台配置 SMTP 服务器设置
   - 将 SMTP 服务器地址设置为本服务的地址（例如：`localhost:2525`）
   - 确保用户的邮箱地址与配置文件中的映射一致

## 安装

```bash
git clone https://github.com/zacharyjia/forgejo-dingtalk.git
cd forgejo-dingtalk
go build
```

## 配置

首次运行时会自动生成示例配置文件 `config.json`：

```json
{
  "dingtalk": {
    "app_key": "dingxxxxxxxxxx",
    "app_secret": "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
    "agent_id": "123456789"
  },
  "smtp": {
    "listen_addr": ":2525",
    "domain": "mail.example.com"
  },
  "user_mappings": {
    "user@example.com": "13800138000",
    "another-user@example.com": "13900139000"
  }
}
```

配置说明：
- `dingtalk.app_key`: 钉钉应用的 AppKey
- `dingtalk.app_secret`: 钉钉应用的 AppSecret
- `dingtalk.agent_id`: 钉钉应用的 AgentId
- `smtp.listen_addr`: SMTP 服务器监听地址和端口
- `smtp.domain`: SMTP 服务器域名（用于 SMTP 握手）
- `user_mappings`: 用户邮箱地址到钉钉用户手机号的映射关系

## 运行

```bash
./forgejo-dingtalk -config config.json
```

参数说明：
- `-config`: 配置文件路径，默认 `config.json`

## 通知内容

服务会自动解析邮件内容，并按以下格式发送钉钉通知：

- 标题：使用邮件主题
- 发件人：显示原始发件人
- 时间：通知发送时间
- 正文：保持邮件原有格式

## 工作原理

1. 服务启动一个 SMTP 服务器
2. Forgejo 将通知以邮件形式发送到该 SMTP 服务器
3. 服务解析邮件内容，提取收件人地址
4. 根据配置的映射关系，找到对应的钉钉用户
5. 将邮件内容转换为 Markdown 格式，发送给对应的钉钉用户

## 开发计划

- [ ] 支持自定义通知模板
- [ ] 支持群聊通知
- [ ] 支持通知消息的分类过滤
- [ ] 添加 Web 管理界面
- [ ] 支持 SSL/TLS

## 贡献

欢迎提交 Issue 和 Pull Request！

## 许可证

MIT License