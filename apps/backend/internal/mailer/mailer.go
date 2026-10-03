package mailer

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/mail"
	"net/smtp"
	"strconv"
	"strings"
	"time"
)

const smtpTimeout = 10 * time.Second

var (
	ErrDisabled = errors.New("smtp delivery is disabled")
	errDelivery = errors.New("smtp delivery failed")
)

type Config struct {
	Host       string
	Port       int
	Username   string
	Password   string
	From       string
	RequireTLS bool
}

type SMTP struct {
	config   Config
	disabled bool
}

func New(config Config) (*SMTP, error) {
	if config.Port == 0 {
		config.Port = 587
	}
	if config.Port < 1 || config.Port > 65535 {
		return nil, fmt.Errorf("SMTP_PORT must be a valid TCP port")
	}

	if config.Host == "" && config.From == "" && config.Username == "" && config.Password == "" {
		return &SMTP{config: config, disabled: true}, nil
	}
	if config.Host == "" {
		return nil, fmt.Errorf("SMTP_HOST is required when SMTP is configured")
	}
	if config.From == "" {
		return nil, fmt.Errorf("SMTP_FROM is required when SMTP is configured")
	}
	if (config.Username == "") != (config.Password == "") {
		return nil, fmt.Errorf("SMTP_USERNAME and SMTP_PASSWORD must be configured together")
	}
	if err := validateHost(config.Host); err != nil {
		return nil, err
	}
	if err := validateMailbox(config.From, "SMTP_FROM"); err != nil {
		return nil, err
	}
	if containsCRLF(config.Username) || containsCRLF(config.Password) {
		return nil, fmt.Errorf("SMTP credentials must not contain CRLF")
	}
	if !config.RequireTLS && !isLoopbackHost(config.Host) {
		return nil, fmt.Errorf("SMTP_REQUIRE_TLS=false is only allowed for a local SMTP host")
	}
	return &SMTP{config: config}, nil
}

func (smtpClient *SMTP) SendPasswordOTP(ctx context.Context, email, otp string) error {
	if smtpClient == nil || smtpClient.disabled {
		return ErrDisabled
	}
	if ctx == nil {
		return errors.New("SMTP context is required")
	}
	if err := validateMailbox(email, "SMTP recipient"); err != nil {
		return err
	}
	if !validOTP(otp) {
		return errors.New("SMTP OTP must be exactly six numeric digits")
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	message := []byte(buildMessage(smtpClient.config.From, email, otp))
	address := net.JoinHostPort(smtpClient.config.Host, strconv.Itoa(smtpClient.config.Port))
	dialer := net.Dialer{Timeout: smtpTimeout}
	connection, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return deliveryError(ctx)
	}
	defer connection.Close()
	if err := setConnectionDeadline(ctx, connection); err != nil {
		return err
	}
	stopWatcher := closeConnectionOnCancel(ctx, connection)
	defer stopWatcher()

	client, err := smtp.NewClient(connection, smtpClient.config.Host)
	if err != nil {
		return deliveryError(ctx)
	}
	defer client.Close()

	startTLS, _ := client.Extension("STARTTLS")
	if startTLS {
		if err := client.StartTLS(&tls.Config{
			MinVersion: tls.VersionTLS12,
			ServerName: smtpClient.config.Host,
		}); err != nil {
			return deliveryError(ctx)
		}
	} else if smtpClient.config.RequireTLS {
		return errors.New("SMTP server does not support required STARTTLS")
	}

	if smtpClient.config.Username != "" {
		if err := client.Auth(smtp.PlainAuth("", smtpClient.config.Username, smtpClient.config.Password, smtpClient.config.Host)); err != nil {
			return deliveryError(ctx)
		}
	}
	if err := client.Mail(smtpClient.config.From); err != nil {
		return deliveryError(ctx)
	}
	if err := client.Rcpt(email); err != nil {
		return deliveryError(ctx)
	}
	writer, err := client.Data()
	if err != nil {
		return deliveryError(ctx)
	}
	if _, err := writer.Write(message); err != nil {
		return deliveryError(ctx)
	}
	if err := writer.Close(); err != nil {
		return deliveryError(ctx)
	}
	if err := client.Quit(); err != nil {
		return deliveryError(ctx)
	}
	return nil
}

func validateHost(host string) error {
	if host == "" || containsCRLF(host) || strings.ContainsAny(host, " \t") || strings.Contains(host, "/") {
		return errors.New("SMTP_HOST is invalid")
	}
	if net.ParseIP(host) == nil && strings.Contains(host, ":") {
		return errors.New("SMTP_HOST must not include a port")
	}
	return nil
}

func containsCRLF(value string) bool {
	return strings.ContainsAny(value, "\r\n")
}

func validateMailbox(value, field string) error {
	if value == "" || containsCRLF(value) || strings.TrimSpace(value) != value {
		return fmt.Errorf("%s is invalid", field)
	}
	parsed, err := mail.ParseAddress(value)
	if err != nil || parsed.Address != value || parsed.Name != "" {
		return fmt.Errorf("%s is invalid", field)
	}
	return nil
}

func validOTP(otp string) bool {
	if len(otp) != 6 {
		return false
	}
	for _, digit := range otp {
		if digit < '0' || digit > '9' {
			return false
		}
	}
	return true
}

func buildMessage(from, recipient, otp string) string {
	return "From: " + from + "\r\n" +
		"To: " + recipient + "\r\n" +
		"Subject: Flood Watch verification code\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: text/plain; charset=UTF-8\r\n" +
		"\r\n" +
		"Your Flood Watch verification code is " + otp + ".\r\n" +
		"This code is useful only for a valid eligible account. If you did not request this code, you can ignore this message.\r\n"
}

func isLoopbackHost(host string) bool {
	host = strings.Trim(host, "[]")
	if strings.EqualFold(strings.TrimSuffix(host, "."), "localhost") {
		return true
	}
	parsed := net.ParseIP(host)
	return parsed != nil && parsed.IsLoopback()
}

func setConnectionDeadline(ctx context.Context, connection net.Conn) error {
	deadline := time.Now().Add(smtpTimeout)
	if contextDeadline, ok := ctx.Deadline(); ok && contextDeadline.Before(deadline) {
		deadline = contextDeadline
	}
	if err := connection.SetDeadline(deadline); err != nil {
		return deliveryError(ctx)
	}
	return nil
}

func closeConnectionOnCancel(ctx context.Context, connection net.Conn) func() {
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = connection.Close()
		case <-done:
		}
	}()
	return func() { close(done) }
}

func deliveryError(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return errDelivery
}
