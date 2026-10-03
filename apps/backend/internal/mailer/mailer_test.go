package mailer

import (
	"bufio"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestNewRejectsInvalidSMTPConfiguration(t *testing.T) {
	for _, test := range []struct {
		name string
		cfg  Config
	}{
		{
			name: "missing host",
			cfg:  Config{Port: 587, From: "no-reply@example.test", RequireTLS: true},
		},
		{
			name: "invalid from",
			cfg:  Config{Host: "localhost", Port: 587, From: "sender\r\nBcc: attacker@example.test", RequireTLS: true},
		},
		{
			name: "from with surrounding whitespace",
			cfg:  Config{Host: "localhost", Port: 587, From: " no-reply@example.test", RequireTLS: true},
		},
		{
			name: "plaintext non-loopback",
			cfg:  Config{Host: "smtp.example.test", Port: 587, From: "no-reply@example.test", RequireTLS: false},
		},
		{
			name: "partial credentials",
			cfg:  Config{Host: "localhost", Port: 587, Username: "user", From: "no-reply@example.test", RequireTLS: false},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := New(test.cfg); err == nil {
				t.Fatal("New() error = nil, want validation error")
			}
		})
	}
}

func TestSendPasswordOTPRejectsInvalidMessageWithoutLeakingValues(t *testing.T) {
	sender, err := New(Config{Host: "127.0.0.1", Port: 1, From: "no-reply@example.test", RequireTLS: false})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	for _, test := range []struct {
		name  string
		email string
		otp   string
	}{
		{name: "invalid recipient", email: "recipient\r\nBcc: attacker@example.test", otp: "123456"},
		{name: "short otp", email: "recipient@example.test", otp: "12345"},
		{name: "non numeric otp", email: "recipient@example.test", otp: "12a456"},
		{name: "otp with line break", email: "recipient@example.test", otp: "123\r\n456"},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := sender.SendPasswordOTP(context.Background(), test.email, test.otp)
			if err == nil {
				t.Fatal("SendPasswordOTP() error = nil, want validation error")
			}
			if strings.Contains(err.Error(), test.otp) {
				t.Fatalf("error %q leaks OTP", err)
			}
		})
	}
}

func TestSendPasswordOTPSendsGenericMessageWithCode(t *testing.T) {
	server := newFakeSMTPServer(t, false)
	sender, err := New(Config{Host: "127.0.0.1", Port: server.port, From: "no-reply@example.test", RequireTLS: false})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if err := sender.SendPasswordOTP(context.Background(), "recipient@example.test", "618204"); err != nil {
		t.Fatalf("SendPasswordOTP() error = %v", err)
	}
	message := server.message(t)
	if !strings.Contains(message, "618204") {
		t.Fatalf("message does not contain OTP: %q", message)
	}
	if !strings.Contains(message, "valid eligible account") {
		t.Fatalf("message is not account-neutral: %q", message)
	}
}

func TestSendPasswordOTPRejectsSMTPWithoutSTARTTLSWhenRequired(t *testing.T) {
	server := newFakeSMTPServer(t, false)
	sender, err := New(Config{Host: "127.0.0.1", Port: server.port, From: "no-reply@example.test", RequireTLS: true})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	err = sender.SendPasswordOTP(context.Background(), "recipient@example.test", "618204")
	if err == nil {
		t.Fatal("SendPasswordOTP() error = nil, want STARTTLS error")
	}
	if strings.Contains(err.Error(), "618204") {
		t.Fatalf("error %q leaks OTP", err)
	}
}

func TestSendPasswordOTPHonorsContextCancellationDuringGreeting(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen() error = %v", err)
	}
	release := make(chan struct{})
	defer close(release)
	defer listener.Close()
	go func() {
		connection, acceptErr := listener.Accept()
		if acceptErr == nil {
			defer connection.Close()
			<-release
		}
	}()

	address := listener.Addr().(*net.TCPAddr)
	sender, err := New(Config{Host: "127.0.0.1", Port: address.Port, From: "no-reply@example.test", RequireTLS: false})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err = sender.SendPasswordOTP(ctx, "recipient@example.test", "618204")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("SendPasswordOTP() error = %v, want context deadline", err)
	}
}

func TestSendPasswordOTPAuthenticatesLocalSMTP(t *testing.T) {
	server := newFakeSMTPServer(t, true)
	sender, err := New(Config{
		Host:       "127.0.0.1",
		Port:       server.port,
		Username:   "smtp-user",
		Password:   "smtp-password",
		From:       "no-reply@example.test",
		RequireTLS: false,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if err := sender.SendPasswordOTP(context.Background(), "recipient@example.test", "618204"); err != nil {
		t.Fatalf("SendPasswordOTP() error = %v", err)
	}
	if !server.authenticated {
		t.Fatal("SMTP server did not receive authentication")
	}
}

type fakeSMTPServer struct {
	listener      net.Listener
	port          int
	withAuth      bool
	authenticated bool
	messageCh     chan string
	errorCh       chan error
}

func newFakeSMTPServer(t *testing.T, withAuth bool) *fakeSMTPServer {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen() error = %v", err)
	}
	server := &fakeSMTPServer{
		listener:  listener,
		port:      listener.Addr().(*net.TCPAddr).Port,
		withAuth:  withAuth,
		messageCh: make(chan string, 1),
		errorCh:   make(chan error, 1),
	}
	t.Cleanup(func() { _ = listener.Close() })
	go server.serve()
	return server
}

func (server *fakeSMTPServer) serve() {
	connection, err := server.listener.Accept()
	if err != nil {
		server.errorCh <- err
		return
	}
	defer connection.Close()
	reader := bufio.NewReader(connection)
	writer := bufio.NewWriter(connection)
	writeLine := func(line string) error {
		if _, err := fmt.Fprintf(writer, "%s\r\n", line); err != nil {
			return err
		}
		return writer.Flush()
	}
	if err := writeLine("220 fake SMTP ready"); err != nil {
		server.errorCh <- err
		return
	}
	var message strings.Builder
	for {
		line, readErr := reader.ReadString('\n')
		if readErr != nil {
			server.errorCh <- readErr
			return
		}
		line = strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
		switch {
		case strings.HasPrefix(line, "EHLO"), strings.HasPrefix(line, "HELO"):
			if err := writeLine("250-localhost"); err != nil {
				server.errorCh <- err
				return
			}
			if server.withAuth {
				if err := writeLine("250-AUTH PLAIN"); err != nil {
					server.errorCh <- err
					return
				}
			}
			if err := writeLine("250 OK"); err != nil {
				server.errorCh <- err
				return
			}
		case strings.HasPrefix(line, "AUTH PLAIN"):
			encoded := strings.TrimSpace(strings.TrimPrefix(line, "AUTH PLAIN"))
			decoded, decodeErr := base64.StdEncoding.DecodeString(encoded)
			if decodeErr != nil || !strings.Contains(string(decoded), "smtp-user") {
				server.errorCh <- fmt.Errorf("invalid auth")
				return
			}
			server.authenticated = true
			if err := writeLine("235 2.7.0 authenticated"); err != nil {
				server.errorCh <- err
				return
			}
		case strings.HasPrefix(line, "MAIL FROM:"):
			if err := writeLine("250 2.1.0 sender ok"); err != nil {
				server.errorCh <- err
				return
			}
		case strings.HasPrefix(line, "RCPT TO:"):
			if err := writeLine("250 2.1.5 recipient ok"); err != nil {
				server.errorCh <- err
				return
			}
		case line == "DATA":
			if err := writeLine("354 end with <CRLF>.<CRLF>"); err != nil {
				server.errorCh <- err
				return
			}
			for {
				dataLine, dataErr := reader.ReadString('\n')
				if dataErr != nil {
					server.errorCh <- dataErr
					return
				}
				if dataLine == ".\r\n" {
					break
				}
				message.WriteString(dataLine)
			}
			server.messageCh <- message.String()
			if err := writeLine("250 2.0.0 queued"); err != nil {
				server.errorCh <- err
				return
			}
		case line == "QUIT":
			_ = writeLine("221 2.0.0 closing connection")
			return
		default:
			if err := writeLine("250 OK"); err != nil {
				server.errorCh <- err
				return
			}
		}
	}
}

func (server *fakeSMTPServer) message(t *testing.T) string {
	t.Helper()
	select {
	case message := <-server.messageCh:
		return message
	case err := <-server.errorCh:
		t.Fatalf("fake SMTP server error = %v", err)
		return ""
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for fake SMTP message")
		return ""
	}
}

func (server *fakeSMTPServer) address() string {
	return net.JoinHostPort("127.0.0.1", strconv.Itoa(server.port))
}
