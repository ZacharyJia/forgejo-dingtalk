package smtp

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"net/textproto"
	"strings"
	"time"

	html2md "github.com/JohannesKaufmann/html-to-markdown/v2"
	"github.com/emersion/go-smtp"
	"github.com/zacharyjia/forgejo-dingtalk/internal/config"
	"github.com/zacharyjia/forgejo-dingtalk/internal/dingtalk"
)

// Backend implements SMTP server methods.
type Backend struct {
	config   *config.Config
	dingtalk *dingtalk.Client
}

// NewBackend creates a new SMTP backend.
func NewBackend(cfg *config.Config, dt *dingtalk.Client) *Backend {
	return &Backend{
		config:   cfg,
		dingtalk: dt,
	}
}

// NewSession is called after EHLO command to create a new SMTP session.
func (bkd *Backend) NewSession(_ *smtp.Conn) (smtp.Session, error) {
	return &Session{
		backend: bkd,
		from:    "",
		to:      make([]string, 0),
	}, nil
}

// Session represents a SMTP session.
type Session struct {
	backend *Backend
	from    string
	to      []string
}

// AuthPlain implements auth plain command - we don't need auth for internal use
func (s *Session) AuthPlain(username, password string) error {
	return nil
}

// Mail implements MAIL command
func (s *Session) Mail(from string, opts *smtp.MailOptions) error {
	s.from = from
	return nil
}

// Rcpt implements RCPT command
func (s *Session) Rcpt(to string, opts *smtp.RcptOptions) error {
	s.to = append(s.to, to)
	return nil
}

// Data implements DATA command
func (s *Session) Data(r io.Reader) error {
	// 读取邮件内容
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		log.Printf("读取邮件内容失败: %v", err)
		return err
	}

	// 解析邮件内容
	content := buf.String()
	subject, body := parseEmailContent(content)

	// 记录原始邮件内容用于调试
	// log.Printf("收到邮件:\nSubject: %s\nFrom: %s\nTo: %v\nContent:\n%s",
	// 	subject, s.from, s.to, content)

	// 收集所有需要发送的用户ID
	var dingTalkIDs []string
	failedEmails := make(map[string]error)

	// 首先收集所有的钉钉用户ID
	for _, to := range s.to {
		// 提取邮件地址
		emailAddr := extractEmailAddress(to)

		// 查找钉钉用户ID
		mobile, ok := s.backend.config.UserMappings[emailAddr]
		if !ok {
			log.Printf("未找到用户映射: %s", emailAddr)
			failedEmails[emailAddr] = fmt.Errorf("未找到用户映射")
			continue
		}

		dingTalkID, err := s.backend.dingtalk.GetUserIdByMobile(mobile)
		if err != nil {
			log.Printf("获取用户ID失败 (mobile: %s): %v", mobile, err)
			failedEmails[emailAddr] = fmt.Errorf("获取用户ID失败: %v", err)
			continue
		}

		dingTalkIDs = append(dingTalkIDs, dingTalkID)
	}

	// 如果有有效的接收者，则发送消息
	if len(dingTalkIDs) > 0 {
		// 构造钉钉消息
		msg := formatDingTalkMessage(subject, body, s.from)

		// 将所有用户ID合并成逗号分隔的字符串
		userIDList := strings.Join(dingTalkIDs, ",")

		// 批量发送钉钉消息
		err := s.backend.dingtalk.SendMessage(userIDList, msg)
		if err != nil {
			log.Printf("批量发送钉钉消息失败: %v", err)
			return err
		}

		log.Printf("成功发送钉钉消息到 %d 个用户", len(dingTalkIDs))
	}

	// 如果有失败的邮件地址，记录日志
	for email, err := range failedEmails {
		log.Printf("发送失败 (to: %s): %v", email, err)
	}

	return nil
}

// Reset implements RSET command
func (s *Session) Reset() {
	s.from = ""
	s.to = nil
}

// Logout implements QUIT command
func (s *Session) Logout() error {
	return nil
}

// extractEmailAddress 从邮件地址中提取纯地址部分
func extractEmailAddress(addr string) string {
	addr = strings.TrimSpace(addr)
	if start := strings.Index(addr, "<"); start != -1 {
		if end := strings.Index(addr[start:], ">"); end != -1 {
			return addr[start+1 : start+end]
		}
	}
	return addr
}

// parseEmailContent 解析邮件内容，提取主题和正文
func parseEmailContent(content string) (subject string, body string) {
	// 解析邮件消息
	msg, err := mail.ReadMessage(bytes.NewReader([]byte(content)))
	if err != nil {
		log.Printf("解析邮件失败: %v", err)
		return "", content
	}

	// 获取主题
	subject = msg.Header.Get("Subject")
	if s, err := (&mime.WordDecoder{}).DecodeHeader(subject); err == nil {
		subject = s
	}

	// 获取 Content-Type
	mediaType, params, err := mime.ParseMediaType(msg.Header.Get("Content-Type"))
	if err != nil {
		log.Printf("解析 Content-Type 失败: %v", err)
		// 如果无法解析 Content-Type，直接读取内容
		rawBody, _ := io.ReadAll(msg.Body)
		return subject, string(rawBody)
	}

	// 处理多部分消息
	if strings.HasPrefix(mediaType, "multipart/") {
		boundary := params["boundary"]
		if boundary != "" {
			body = handleMultipartMessage(msg.Body, boundary)
			return
		}
	}

	// 处理单一部分消息
	body = handleSinglePartMessage(msg.Body, convertMailHeaderToMIME(msg.Header))
	return
}

// handleMultipartMessage 处理多部分消息
func handleMultipartMessage(reader io.Reader, boundary string) string {
	mr := multipart.NewReader(reader, boundary)
	var htmlContent, textContent string

	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Printf("读取多部分消息失败: %v", err)
			break
		}

		mediaType, _, err := mime.ParseMediaType(part.Header.Get("Content-Type"))
		if err != nil {
			continue
		}
		content := handleSinglePartMessage(part, part.Header)

		switch mediaType {
		case "text/html":
			// 将HTML转换为Markdown
			if markdown, err := html2md.ConvertString(content); err == nil {
				htmlContent = markdown
			} else {
				log.Printf("转换HTML到Markdown失败: %v", err)
				htmlContent = content
			}
		case "text/plain":
			textContent = content
		}
	}
	// 优先使用 HTML 内容
	if htmlContent != "" {
		return htmlContent
	}
	return textContent
}

// handleSinglePartMessage 处理单一部分消息
func handleSinglePartMessage(reader io.Reader, header textproto.MIMEHeader) string {
	// 处理内容传输编码
	encoding := header.Get("Content-Transfer-Encoding")
	var r io.Reader = reader

	switch strings.ToLower(encoding) {
	case "quoted-printable":
		r = quotedprintable.NewReader(reader)
	case "base64":
		r = base64.NewDecoder(base64.StdEncoding, reader)
	}

	content, err := io.ReadAll(r)
	if err != nil {
		log.Printf("读取内容失败: %v", err)
		return ""
	}

	// 处理字符集编码
	contentType := header.Get("Content-Type")
	if _, params, err := mime.ParseMediaType(contentType); err == nil {
		if charset := params["charset"]; charset != "" {
			if decoded, err := decodeCharset(content, charset); err == nil {
				return decoded
			}
		}
	}

	return string(content)
}

// convertMailHeaderToMIME 将mail.Header转换为textproto.MIMEHeader
func convertMailHeaderToMIME(header mail.Header) textproto.MIMEHeader {
	mime := make(textproto.MIMEHeader)
	for k, v := range header {
		mime[k] = v
	}
	return mime
}

// decodeCharset 解码不同字符集的内容
func decodeCharset(content []byte, charset string) (string, error) {
	charset = strings.ToLower(charset)

	// 目前只支持 UTF-8 和 ASCII
	// 其他编码先返回原始内容，后续可以添加更多字符集支持
	switch charset {
	case "utf-8", "us-ascii":
		return string(content), nil
	case "gb2312", "gbk", "gb18030":
		log.Printf("暂不支持的中文编码: %s", charset)
	default:
		log.Printf("未知的字符集编码: %s", charset)
	}

	return string(content), nil
}

// formatDingTalkMessage 格式化钉钉消息
func formatDingTalkMessage(subject, body, from string) string {
	var builder strings.Builder

	// 添加标题
	builder.WriteString("## ")
	builder.WriteString(subject)
	builder.WriteString("\n\n")

	// 添加发件人信息
	// builder.WriteString("**From:** ")
	// builder.WriteString(from)
	// builder.WriteString("\n\n")

	// 添加时间
	builder.WriteString("**Time:** ")
	builder.WriteString(time.Now().Format("2006-01-02 15:04:05"))
	builder.WriteString("\n\n")

	// 添加正文
	builder.WriteString("---\n\n")
	builder.WriteString(body)

	return builder.String()
}

// Server 表示SMTP服务器
type Server struct {
	backend *Backend
	server  *smtp.Server
}

// NewServer 创建新的SMTP服务器
func NewServer(cfg *config.Config, dt *dingtalk.Client) *Server {
	backend := NewBackend(cfg, dt)

	srv := smtp.NewServer(backend)
	srv.Addr = cfg.SMTP.ListenAddr
	srv.Domain = cfg.SMTP.Domain
	srv.ReadTimeout = 10 * time.Second
	srv.WriteTimeout = 10 * time.Second
	srv.MaxMessageBytes = 1024 * 1024 // 1MB
	srv.MaxRecipients = 50
	srv.AllowInsecureAuth = true

	return &Server{
		backend: backend,
		server:  srv,
	}
}

// Start 启动SMTP服务器
func (s *Server) Start() error {
	log.Printf("SMTP服务器正在监听: %s", s.server.Addr)
	return s.server.ListenAndServe()
}

// Stop 停止SMTP服务器
func (s *Server) Stop() error {
	return s.server.Close()
}
