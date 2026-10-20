package email

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"strings"
	"sync"
	"time"
)

type EmailData struct {
	To      string
	Subject string
	Body    string
}

type EmailSender func(ctx context.Context, data EmailData) error

type SMTPSender struct {
	host     string
	port     int
	user     string
	pass     string
	tlsMode  string
	from     string
	auth     smtp.Auth
	dialFunc func(network, addr string) (net.Conn, error)
	mu       sync.Mutex
	client   *smtp.Client
	conn     net.Conn
}

func NewSMTPSender(host string, port int, user, pass, tlsMode, from string) EmailSender {
	s := &SMTPSender{
		host:    host,
		port:    port,
		user:    user,
		pass:    pass,
		tlsMode: tlsMode,
		from:    from,
	}
	if user != "" && pass != "" {
		s.auth = smtp.PlainAuth("", user, pass, host)
	}
	s.initDialFunc()
	return s.Send
}

func (s *SMTPSender) Send(ctx context.Context, data EmailData) error {
	if s.host == "" {
		return nil
	}

	addr := fmt.Sprintf("%s:%d", s.host, s.port)
	msg := composeMessage(s.from, data.To, data.Subject, data.Body)

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.ensureConnection(addr); err != nil {
		return err
	}

	if err := s.sendEmail(data.To, msg); err != nil {
		s.closeConnection()
		return err
	}

	return nil
}

func (s *SMTPSender) ensureConnection(addr string) error {
	if s.client != nil {
		return nil
	}

	conn, err := s.dialFunc("tcp", addr)
	if err != nil {
		return err
	}

	client, err := smtp.NewClient(conn, s.host)
	if err != nil {
		conn.Close()
		return err
	}

	if s.auth != nil {
		if err := client.Auth(s.auth); err != nil {
			client.Close()
			conn.Close()
			return err
		}
	}

	s.conn = conn
	s.client = client
	return nil
}

func (s *SMTPSender) sendEmail(to, msg string) error {
	if err := s.client.Mail(s.from); err != nil {
		return err
	}
	if err := s.client.Rcpt(to); err != nil {
		return err
	}
	w, err := s.client.Data()
	if err != nil {
		return err
	}
	_, err = w.Write([]byte(msg))
	if err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return nil
}

func (s *SMTPSender) closeConnection() {
	if s.client != nil {
		s.client.Quit()
		s.client = nil
	}
	if s.conn != nil {
		s.conn.Close()
		s.conn = nil
	}
}

func (s *SMTPSender) initDialFunc() {
	switch s.tlsMode {
	case "tls":
		s.dialFunc = func(network, addr string) (net.Conn, error) {
			return tls.DialWithDialer(&net.Dialer{Timeout: 10 * time.Second}, network, addr, &tls.Config{ServerName: s.host})
		}
	case "starttls":
		s.dialFunc = func(network, addr string) (net.Conn, error) {
			conn, err := net.DialTimeout(network, addr, 10*time.Second)
			if err != nil {
				return nil, err
			}
			client, err := smtp.NewClient(conn, s.host)
			if err != nil {
				conn.Close()
				return nil, err
			}
			if err := client.StartTLS(&tls.Config{ServerName: s.host}); err != nil {
				client.Close()
				return nil, err
			}
			return conn, nil
		}
	default:
		s.dialFunc = func(network, addr string) (net.Conn, error) {
			return net.DialTimeout(network, addr, 10*time.Second)
		}
	}
}

func composeMessage(from, to, subject, body string) string {
	var sb strings.Builder
	sb.WriteString("From: " + from + "\r\n")
	sb.WriteString("To: " + to + "\r\n")
	sb.WriteString("Subject: " + subject + "\r\n")
	sb.WriteString("MIME-Version: 1.0\r\n")
	sb.WriteString("Content-Type: text/plain; charset=\"utf-8\"\r\n")
	sb.WriteString("\r\n")
	sb.WriteString(body)
	return sb.String()
}
