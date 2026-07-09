package net

import (
	"context"
	"fmt"
	"net"
	"time"
)

type Resolver struct {
	resolver *net.Resolver
	timeout  time.Duration
}

func NewResolver(timeout time.Duration) *Resolver {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &Resolver{
		resolver: net.DefaultResolver,
		timeout:  timeout,
	}
}

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

type DNSResult struct {
	Hostname  string   `json:"hostname"`
	Addresses []string `json:"addresses"`
	CNAME     string   `json:"cname,omitempty"`
}

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

func DefaultResolveHost(ctx context.Context, hostname string) (*DNSResult, error) {
	r := &Resolver{resolver: net.DefaultResolver, timeout: 5 * time.Second}
	return r.LookupHost(ctx, hostname)
}
