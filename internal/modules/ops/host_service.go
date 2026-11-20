package ops

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/Allinost/go-backend-core/internal/services/auth"
	"github.com/Allinost/go-backend-core/internal/services/crypto"
	"golang.org/x/crypto/ssh"
)

type HostService struct {
	repo   *HostRepository
	cipher *crypto.AESGCM
}

func NewHostService(repo *HostRepository, cipher *crypto.AESGCM) *HostService {
	return &HostService{repo: repo, cipher: cipher}
}

func (s *HostService) List(ctx context.Context, req *ListHostsReq) ([]Host, bool, int, error) {
	if req.Role != "" && !ValidHostRoles[req.Role] {
		return nil, false, 0, hostValidationError("无效角色")
	}
	if req.Environment != "" && !ValidHostEnvs[req.Environment] {
		return nil, false, 0, hostValidationError("无效环境")
	}
	return s.repo.List(ctx, req)
}

func (s *HostService) Get(ctx context.Context, id int64) (*Host, error) {
	return s.repo.Get(ctx, id)
}

func (s *HostService) Create(ctx context.Context, req *CreateHostReq, actor string) (*Host, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, hostValidationError("主机名不能为空")
	}
	if !ValidHostRoles[req.Role] {
		return nil, hostValidationError("无效角色")
	}
	env := req.Environment
	if env == "" {
		env = "prod"
	}
	if !ValidHostEnvs[env] {
		return nil, hostValidationError("无效环境")
	}
	address := strings.TrimSpace(req.Address)
	if address == "" {
		return nil, hostValidationError("地址不能为空")
	}
	port := req.SSHPort
	if port == 0 {
		port = 22
	}
	if port < 1 || port > 65535 {
		return nil, hostValidationError("SSH 端口非法")
	}
	authType := req.AuthType
	if authType == "" {
		authType = "password"
	}
	if !ValidHostAuthTypes[authType] {
		return nil, hostValidationError("认证方式应为 password/key")
	}
	username := strings.TrimSpace(req.Username)
	if username == "" {
		username = "root"
	}
	secret := req.Password
	if authType == "key" {
		secret = req.PrivateKey
	}
	ciphertext, hint, err := s.sealCredential(authType, secret)
	if err != nil {
		return nil, err
	}
	h := &Host{
		Name: name, Role: req.Role, Environment: env, Address: address, SSHPort: port, Username: username,
		AuthType: authType, CredentialHint: hint, OS: req.OS, Tags: req.Tags, Notes: req.Notes,
		Status: HostStatusUnknown, CreatedBy: actor, UpdatedBy: actor, credentialCiphertext: ciphertext,
	}
	return s.repo.Create(ctx, h)
}

func (s *HostService) Update(ctx context.Context, id int64, req *UpdateHostReq, actor string) (*Host, error) {
	updates := map[string]any{}
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return nil, hostValidationError("主机名不能为空")
		}
		updates["name"] = name
	}
	if req.Role != nil {
		if !ValidHostRoles[*req.Role] {
			return nil, hostValidationError("无效角色")
		}
		updates["role"] = *req.Role
	}
	if req.Environment != nil {
		if !ValidHostEnvs[*req.Environment] {
			return nil, hostValidationError("无效环境")
		}
		updates["environment"] = *req.Environment
	}
	if req.Address != nil {
		address := strings.TrimSpace(*req.Address)
		if address == "" {
			return nil, hostValidationError("地址不能为空")
		}
		updates["address"] = address
	}
	if req.SSHPort != nil {
		if *req.SSHPort < 1 || *req.SSHPort > 65535 {
			return nil, hostValidationError("SSH 端口非法")
		}
		updates["ssh_port"] = *req.SSHPort
	}
	if req.Username != nil {
		updates["username"] = strings.TrimSpace(*req.Username)
	}
	if req.AuthType != nil {
		if !ValidHostAuthTypes[*req.AuthType] {
			return nil, hostValidationError("认证方式应为 password/key")
		}
		updates["auth_type"] = *req.AuthType
	}
	if req.OS != nil {
		updates["os"] = *req.OS
	}
	if req.Tags != nil {
		updates["tags"] = mustJSON(req.Tags)
	}
	if req.Notes != nil {
		updates["notes"] = *req.Notes
	}
	authType := "password"
	if v, ok := updates["auth_type"].(string); ok {
		authType = v
	}
	secret := ""
	if req.Password != nil {
		secret = *req.Password
	}
	if req.PrivateKey != nil {
		secret = *req.PrivateKey
	}
	if secret != "" {
		ciphertext, hint, err := s.sealCredential(authType, secret)
		if err != nil {
			return nil, err
		}
		updates["credential_ciphertext"] = ciphertext
		updates["credential_hint"] = hint
	}
	return s.repo.Update(ctx, id, updates, actor)
}

func (s *HostService) Delete(ctx context.Context, id int64, actor string) error {
	return s.repo.SoftDelete(ctx, id, actor)
}

func (s *HostService) sealCredential(authType, secret string) (string, string, error) {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return "", "", nil
	}
	if s.cipher == nil {
		return "", "", fmt.Errorf("%w: 未配置 crypto.master_key_hex", ErrHostCredential)
	}
	encrypted, err := s.cipher.EncryptString(secret)
	if err != nil {
		return "", "", fmt.Errorf("%w: %v", ErrHostCredential, err)
	}
	hint := secret
	if authType == "key" {
		sum := sha256.Sum256([]byte(secret))
		hint = "key:" + hex.EncodeToString(sum[:])[:12]
	} else if len(hint) > 4 {
		hint = "****" + hint[len(hint)-4:]
	} else {
		hint = "****"
	}
	return encrypted, hint, nil
}

func (s *HostService) openCredential(h *Host) (string, error) {
	if h.credentialCiphertext == "" {
		return "", fmt.Errorf("%w: 未保存凭据", ErrHostCredential)
	}
	if s.cipher == nil {
		return "", fmt.Errorf("%w: 未配置 crypto.master_key_hex", ErrHostCredential)
	}
	plain, err := s.cipher.DecryptString(h.credentialCiphertext)
	if err != nil {
		return "", fmt.Errorf("%w: 解密失败", ErrHostCredential)
	}
	return plain, nil
}

func (s *HostService) Test(ctx context.Context, id int64) (*HostTestResult, error) {
	h, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	secret, err := s.openCredential(h)
	if err != nil {
		_ = s.repo.UpdateStatus(ctx, id, HostStatusError, err.Error(), time.Now())
		return nil, err
	}
	start := time.Now()
	client, err := dialSSH(ctx, h, secret)
	if err != nil {
		_ = s.repo.UpdateStatus(ctx, id, HostStatusOffline, err.Error(), time.Now())
		return &HostTestResult{Online: false, Status: HostStatusOffline, Error: err.Error(), DurationMS: time.Since(start).Milliseconds()}, nil
	}
	defer client.Close()
	_ = s.repo.UpdateStatus(ctx, id, HostStatusOnline, "", time.Now())
	return &HostTestResult{
		Online: true, Status: HostStatusOnline, DurationMS: time.Since(start).Milliseconds(),
		RemoteAddr: client.RemoteAddr().String(), ServerInfo: string(client.ServerVersion()),
	}, nil
}

func (s *HostService) Status(ctx context.Context, id int64) (*HostStatusResult, error) {
	h, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	secret, err := s.openCredential(h)
	if err != nil {
		return nil, err
	}
	client, err := dialSSH(ctx, h, secret)
	if err != nil {
		_ = s.repo.UpdateStatus(ctx, id, HostStatusOffline, err.Error(), time.Now())
		return &HostStatusResult{Online: false, Status: HostStatusOffline, Error: err.Error(), CollectedAt: time.Now()}, nil
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return nil, err
	}
	defer session.Close()
	out, err := session.CombinedOutput("uptime; echo '---'; df -h /; echo '---'; free -m")
	if err != nil {
		return nil, fmt.Errorf("采集状态失败: %w", err)
	}
	parts := strings.Split(string(out), "---")
	result := &HostStatusResult{Online: true, Status: HostStatusOnline, CollectedAt: time.Now()}
	if len(parts) > 0 {
		result.Uptime = strings.TrimSpace(parts[0])
	}
	if len(parts) > 1 {
		result.Disk = strings.TrimSpace(parts[1])
	}
	if len(parts) > 2 {
		result.Memory = strings.TrimSpace(parts[2])
	}
	_ = s.repo.UpdateStatus(ctx, id, HostStatusOnline, "", time.Now())
	return result, nil
}

func dialSSH(ctx context.Context, h *Host, secret string) (*ssh.Client, error) {
	authType := h.AuthType
	var auth []ssh.AuthMethod
	if authType == "key" {
		signer, err := ssh.ParsePrivateKey([]byte(secret))
		if err != nil {
			return nil, fmt.Errorf("解析私钥失败: %w", err)
		}
		auth = append(auth, ssh.PublicKeys(signer))
	} else {
		auth = append(auth, ssh.Password(secret))
	}
	cfg := &ssh.ClientConfig{
		User:            h.Username,
		Auth:            auth,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // V3.0 先不落 known_hosts
		Timeout:         10 * time.Second,
	}
	addr := net.JoinHostPort(h.Address, fmt.Sprint(h.SSHPort))
	dialer := net.Dialer{Timeout: 10 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}
	c, chans, reqs, err := ssh.NewClientConn(conn, addr, cfg)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	return ssh.NewClient(c, chans, reqs), nil
}

func mustJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// SeedHostPermissions 幂等登记主机台账权限点。
func SeedHostPermissions(rbac *auth.RBACService) error {
	if rbac == nil {
		return fmt.Errorf("ops: RBAC 不可用")
	}
	existing, err := rbac.ListPermissions()
	if err != nil {
		return fmt.Errorf("ops: 读取权限失败: %w", err)
	}
	have := make(map[string]bool, len(existing))
	for _, p := range existing {
		have[p.Name] = true
	}
	specs := map[string]string{
		"ops.host.read":    "查看主机台账",
		"ops.host.manage":  "新建/修改/删除主机",
		"ops.host.connect": "测试 SSH 连通/采集状态",
	}
	for name := range specs {
		if have[name] {
			continue
		}
		action := strings.TrimPrefix(name, "ops.host.")
		if _, err := rbac.CreatePermission(name, "ops.host", action); err != nil {
			return fmt.Errorf("ops: 创建权限 %s 失败: %w", name, err)
		}
	}
	return nil
}
