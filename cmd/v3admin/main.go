// Command v3admin 创建或修复 V3 管理员账号。
// 用法：
//
//	$env:V3_ADMIN_PASSWORD='...'; go run ./cmd/v3admin -username admin -nickname 管理员
//
// 生产构建：go build -o v3admin ./cmd/v3admin
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Allinost/go-backend-core/internal/config"
	"github.com/Allinost/go-backend-core/internal/database/postgres"
	"github.com/Allinost/go-backend-core/internal/services/auth"
)

func main() {
	username := flag.String("username", "admin", "管理员用户名")
	password := flag.String("password", "", "密码；为空时读取 V3_ADMIN_PASSWORD")
	nickname := flag.String("nickname", "管理员", "昵称")
	role := flag.String("role", string(auth.RoleAdmin), "角色：admin 或 user")
	flag.Parse()

	user := strings.TrimSpace(*username)
	if user == "" {
		fail("用户名不能为空")
	}
	pass := *password
	if pass == "" {
		pass = os.Getenv("V3_ADMIN_PASSWORD")
	}
	if len(pass) < 8 {
		fail("密码至少 8 位，请使用 -password 或 V3_ADMIN_PASSWORD 提供")
	}
	if *role != string(auth.RoleAdmin) && *role != string(auth.RoleUser) {
		fail("角色只能是 admin 或 user")
	}

	cfg, err := config.Load("config.yaml")
	if err != nil {
		fail("加载配置失败: " + err.Error())
	}
	pgCfg := cfg.Database.Postgres["main"]
	pool, err := postgres.NewPool(pgCfg, false)
	if err != nil {
		fail("连接 PostgreSQL 失败: " + err.Error())
	}
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var usersTable bool
	if err := pool.QueryRow(ctx, `SELECT to_regclass('users') IS NOT NULL`).Scan(&usersTable); err != nil {
		fail("检查表失败: " + err.Error())
	}
	if !usersTable {
		fail("users 表不存在，请先执行迁移 POST /api/v1/migrate/schema/migrate")
	}

	userStore, err := auth.NewPostgresUserStore(pool.SQLDB)
	if err != nil {
		fail("初始化用户存储失败: " + err.Error())
	}
	rbacStore, err := auth.NewPostgresRBACStore(pool.SQLDB)
	if err != nil {
		fail("初始化 RBAC 失败: " + err.Error())
	}
	if err := rbacStore.EnsureDefaultPermissions(); err != nil {
		fail("初始化默认权限失败: " + err.Error())
	}

	account, err := userStore.FindByUsername(user)
	if err != nil {
		account, err = userStore.CreateUser(auth.RegisterRequest{
			Username: user,
			Password: pass,
			Nickname: *nickname,
		})
		if err != nil {
			fail("创建用户失败: " + err.Error())
		}
		fmt.Printf("已创建用户 id=%d username=%s\n", account.ID, account.Username)
	} else {
		fmt.Printf("用户已存在 id=%d username=%s，仅补齐角色\n", account.ID, account.Username)
	}

	if err := rbacStore.AssignRole(account.ID, *role); err != nil {
		fail("分配角色失败: " + err.Error())
	}
	roles, err := rbacStore.GetRoles(account.ID)
	if err != nil {
		fail("读取角色失败: " + err.Error())
	}
	fmt.Printf("完成：username=%s roles=%v\n", account.Username, roles)
	if user == "admin" && *role == string(auth.RoleAdmin) {
		fmt.Println("提示：首次登录后请立即修改密码")
	}
}

func fail(message string) {
	fmt.Fprintln(os.Stderr, "v3admin: "+message)
	os.Exit(1)
}
