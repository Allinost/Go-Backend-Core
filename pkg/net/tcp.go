package net

import (
	"context"
	"fmt"
	"io"
	"net"
	"time"
)

type TCPConfig struct {
	ConnectTimeout time.Duration
	ReadTimeout    time.Duration
	WriteTimeout   time.Duration
}

func DefaultTCPConfig() TCPConfig {
	return TCPConfig{
		ConnectTimeout: 5 * time.Second,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
	}
}

type TCPConn struct {
	conn   net.Conn
	config TCPConfig
}

func DialTCP(ctx context.Context, addr string, cfg TCPConfig) (*TCPConn, error) {
	dialer := &net.Dialer{
		Timeout: cfg.ConnectTimeout,
	}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("net: TCP 连接失败 %s: %w", addr, err)
	}
	return &TCPConn{conn: conn, config: cfg}, nil
}

func (c *TCPConn) Send(data []byte) ([]byte, error) {
	return c.SendWithTimeout(data, c.config.ReadTimeout)
}

func (c *TCPConn) SendWithTimeout(data []byte, timeout time.Duration) ([]byte, error) {
	if err := c.conn.SetWriteDeadline(time.Now().Add(c.config.WriteTimeout)); err != nil {
		return nil, fmt.Errorf("net: 设置写超时失败: %w", err)
	}
	if _, err := c.conn.Write(data); err != nil {
		return nil, fmt.Errorf("net: TCP 发送失败: %w", err)
	}

	if err := c.conn.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		return nil, fmt.Errorf("net: 设置读超时失败: %w", err)
	}
	result, err := io.ReadAll(c.conn)
	if err != nil {
		return nil, fmt.Errorf("net: TCP 读取失败: %w", err)
	}
	return result, nil
}

func (c *TCPConn) Close() error {
	return c.conn.Close()
}

func (c *TCPConn) Raw() net.Conn {
	return c.conn
}

func CheckTCP(ctx context.Context, addr string) error {
	docker := &net.Dialer{Timeout: 5 * time.Second}
	conn, err := docker.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("net: TCP 端口 %s 不可达: %w", addr, err)
	}
	conn.Close()
	return nil
}

func CheckPort(ctx context.Context, host string, port int, timeout time.Duration) bool {
	addr := fmt.Sprintf("%s:%d", host, port)
	docker := &net.Dialer{Timeout: timeout}
	conn, err := docker.DialContext(ctx, "tcp", addr)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func ScanPorts(ctx context.Context, host string, ports []int, timeout time.Duration) map[int]bool {
	result := make(map[int]bool, len(ports))
	for _, port := range ports {
		result[port] = CheckPort(ctx, host, port, timeout)
	}
	return result
}
