package net

import (
	"context"
	"fmt"
	"net"
	"time"
)

type UDPConfig struct {
	ConnectTimeout time.Duration
	ReadTimeout    time.Duration
	ProbeData      []byte
}

func DefaultUDPConfig() UDPConfig {
	return UDPConfig{
		ConnectTimeout: 5 * time.Second,
		ReadTimeout:    3 * time.Second,
		ProbeData:      []byte{0x00},
	}
}

type UDPConn struct {
	conn   *net.UDPConn
	addr   *net.UDPAddr
	config UDPConfig
}

func DialUDP(ctx context.Context, addr string, cfg UDPConfig) (*UDPConn, error) {
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, fmt.Errorf("net: 解析 UDP 地址 %s 失败: %w", addr, err)
	}
	conn, err := net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		return nil, fmt.Errorf("net: UDP 连接 %s 失败: %w", addr, err)
	}
	return &UDPConn{conn: conn, addr: udpAddr, config: cfg}, nil
}

func (c *UDPConn) Send(data []byte) ([]byte, error) {
	if err := c.conn.SetWriteDeadline(time.Now().Add(c.config.ConnectTimeout)); err != nil {
		return nil, fmt.Errorf("net: 设置写超时失败: %w", err)
	}
	if _, err := c.conn.Write(data); err != nil {
		return nil, fmt.Errorf("net: UDP 发送失败: %w", err)
	}

	if err := c.conn.SetReadDeadline(time.Now().Add(c.config.ReadTimeout)); err != nil {
		return nil, fmt.Errorf("net: 设置读超时失败: %w", err)
	}
	buf := make([]byte, 65535)
	n, _, err := c.conn.ReadFromUDP(buf)
	if err != nil {
		return nil, fmt.Errorf("net: UDP 读取失败: %w", err)
	}
	return buf[:n], nil
}

func (c *UDPConn) Close() error {
	return c.conn.Close()
}

func CheckUDP(ctx context.Context, addr string) error {
	dialer := net.Dialer{Timeout: 5 * time.Second}
	conn, err := dialer.DialContext(ctx, "udp", addr)
	if err != nil {
		return fmt.Errorf("net: UDP 连接 %s 失败: %w", addr, err)
	}
	defer conn.Close()

	if _, err := conn.Write([]byte{0x00}); err != nil {
		return fmt.Errorf("net: UDP 探测发送失败: %w", err)
	}

	readErr := make(chan error, 1)
	go func() {
		buf := make([]byte, 1)
		_, err := conn.Read(buf)
		readErr <- err
	}()

	select {
	case err := <-readErr:
		if err != nil {
			return fmt.Errorf("net: UDP 端口 %s 不可达: %w", addr, err)
		}
		return nil
	case <-time.After(2 * time.Second):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
