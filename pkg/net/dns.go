package net

import (
	"context"
	"fmt"
	"net"
	"time"
)

// Resolver DNS 解析器封装，支持超时控制
type Resolver struct {
	resolver *net.Resolver
	timeout  time.Duration
}

// NewResolver 创建使用默认 DNS 的解析器，超时时间默认为 5s
func NewResolver(timeout time.Duration) *Resolver {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &Resolver{
		resolver: net.DefaultResolver,
		timeout:  timeout,
	}
}

// NewResolverWithDNS 创建使用自定义 DNS 服务器的解析器
func NewResolverWithDNS(server string, timeout time.Duration) *Resolver {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &Resolver{
		resolver: &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				d := net.Dialer{Timeout: timeout}
				return d.DialContext(ctx, "udp", server+":53")
			},
		},
		timeout: timeout,
	}
}

// DNSResult DNS 解析结果
type DNSResult struct {
	Hostname  string   `json:"hostname"`
	Addresses []string `json:"addresses"`
	CNAME     string   `json:"cname,omitempty"`
}

// LookupHost 解析主机名对应的 IP 地址和 CNAME 记录
func (r *Resolver) LookupHost(ctx context.Context, hostname string) (*DNSResult, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	addrs, err := r.resolver.LookupHost(ctx, hostname)
	if err != nil {
		return nil, fmt.Errorf("net: DNS 解析 %s 失败: %w", hostname, err)
	}

	cname, _ := r.resolver.LookupCNAME(ctx, hostname)

	return &DNSResult{
		Hostname:  hostname,
		Addresses: addrs,
		CNAME:     cname,
	}, nil
}

// LookupMX 查询主机名的 MX 邮件交换记录
func (r *Resolver) LookupMX(ctx context.Context, hostname string) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	mxRecords, err := r.resolver.LookupMX(ctx, hostname)
	if err != nil {
		return nil, fmt.Errorf("net: MX 查询 %s 失败: %w", hostname, err)
	}

	result := make([]string, len(mxRecords))
	for i, mx := range mxRecords {
		result[i] = fmt.Sprintf("%s (priority=%d)", mx.Host, mx.Pref)
	}
	return result, nil
}

// DefaultResolveHost 使用默认 DNS 解析主机名（5s 超时）
func DefaultResolveHost(ctx context.Context, hostname string) (*DNSResult, error) {
	r := &Resolver{resolver: net.DefaultResolver, timeout: 5 * time.Second}
	return r.LookupHost(ctx, hostname)
}
