# 测试开发规范

## 测试金字塔

```
        ╱╲
       ╱  ╲        E2E 测试（少量）
      ╱    ╲       关键业务流程覆盖
     ╱──────╲
    ╱        ╲     集成测试（适量）
   ╱          ╲    API / DB / 外部依赖
  ╱────────────╲
 ╱              ╲  单元测试（大量）
╱                ╲  Handler / Service / Repository / Pkg
```

| 层级 | 比例 | 运行速度 | 依赖 |
|------|------|----------|------|
| 单元测试 | 70% | ms 级 | 无（mock 外部依赖） |
| 集成测试 | 20% | s 级 | 测试数据库 / Redis |
| E2E 测试 | 10% | 10s+ | 完整服务 + 依赖 |

---

## 技术选型

| 工具 | 用途 |
|------|------|
| **testify** | 断言库 (`assert.Equal`, `require.NoError`) |
| **gomock** | 接口 Mock 生成 (`mockgen`) |
| **httptest** | HTTP handler 测试（标准库） |
| **dockertest** | 集成测试中启动 PostgreSQL / Redis 容器 |
| **go-cmp** | 复杂结构体深度比较 (`cmp.Diff`) |
| **go-sqlmock** | 模拟 SQL 查询（repository 层测试） |
| **miniredis** | 模拟 Redis 操作（无需真实 Redis） |
| **go-faker** | 测试数据生成 |

安装：

```bash
go install github.com/golang/mock/mockgen@latest
```

---

## 目录约定

每个模块的测试文件放在同一包内，命名遵循 Go 惯例：

```
internal/
├── services/
│   └── auth/
│       ├── handler.go
│       ├── handler_test.go      # handler 测试
│       ├── service.go
│       ├── service_test.go      # service 逻辑测试
│       ├── repository.go
│       ├── repository_test.go   # 数据访问测试
│       └── model.go
├── modules/
│   └── s1/
│       ├── handler_test.go
│       ├── service_test.go
│       └── repository_test.go
└── pkg/
    └── crypto/
        └── crypto_test.go       # 纯函数测试
```

---

## 三层架构测试策略

### Handler 层 — HTTP 请求/响应测试

```go
func TestAuthHandler_Login(t *testing.T) {
    // 1. 构造 mock service
    mockSvc := NewMockAuthService(gomock.NewController(t))
    mockSvc.EXPECT().Login(gomock.Any(), "test@example.com", "pass").
        Return(&dto.LoginResponse{Token: "jwt..."}, nil)

    // 2. 创建 handler + 路由
    handler := NewAuthHandler(mockSvc)
    router := gin.New()
    handler.RegisterRoutes(router.Group("/api/v1/auth"))

    // 3. 发起 HTTP 请求
    w := httptest.NewRecorder()
    req, _ := http.NewRequest("POST", "/api/v1/auth/login", jsonBody(t, map[string]any{
        "email":    "test@example.com",
        "password": "pass",
    }))
    router.ServeHTTP(w, req)

    // 4. 断言
    assert.Equal(t, 200, w.Code)
    assert.Contains(t, w.Body.String(), "jwt")
}
```

### Service 层 — 业务逻辑测试

```go
func TestAuthService_Login_Success(t *testing.T) {
    mockRepo := NewMockAuthRepository(ctrl)
    mockRepo.EXPECT().FindByEmail("test@example.com").
        Return(&model.User{ID: 1, PasswordHash: hashedPass}, nil)

    svc := NewAuthService(mockRepo, jwtSecret)
    resp, err := svc.Login(ctx, "test@example.com", "plainPass")

    assert.NoError(t, err)
    assert.NotEmpty(t, resp.Token)
}

func TestAuthService_Login_WrongPassword(t *testing.T) {
    mockRepo := NewMockAuthRepository(ctrl)
    mockRepo.EXPECT().FindByEmail("test@example.com").
        Return(&model.User{ID: 1, PasswordHash: wrongHash}, nil)

    svc := NewAuthService(mockRepo, jwtSecret)
    _, err := svc.Login(ctx, "test@example.com", "plainPass")

    assert.ErrorIs(t, err, ErrInvalidCredentials)
}
```

### Repository 层 — 数据访问测试

```go
// repository_test.go — 使用真实测试数据库
func TestAuthRepository_FindByEmail(t *testing.T) {
    db := setupTestDB(t)                // 初始化测试数据库
    defer teardownTestDB(db)

    repo := NewAuthRepository(db)
    user, err := repo.FindByEmail("existing@example.com")

    assert.NoError(t, err)
    assert.Equal(t, int64(1), user.ID)
}

// 使用 go-sqlmock 模拟（无需真实 DB）
func TestAuthRepository_FindByEmail_NotFound(t *testing.T) {
    db, mock, err := sqlmock.New()
    require.NoError(t, err)

    mock.ExpectQuery(`SELECT .+ FROM users WHERE email = .+`).
        WithArgs("notfound@example.com").
        WillReturnError(sql.ErrNoRows)

    repo := NewAuthRepository(db)
    _, err = repo.FindByEmail("notfound@example.com")
    assert.ErrorIs(t, err, ErrUserNotFound)
}
```

---

## Mock 生成与管理

### 接口 Mock

为所有 Service 接口生成 mock：

```go
//go:generate mockgen -source=service.go -destination=mock_service.go -package=auth

type AuthService interface {
    Login(ctx, email, password string) (*LoginResponse, error)
    Register(ctx, req RegisterRequest) (*User, error)
}
```

运行：

```bash
go generate ./internal/services/auth/...
```

### Mock 使用原则

| 场景 | Mock 方式 |
|------|-----------|
| Service 测试 | Mock Repository 接口 |
| Handler 测试 | Mock Service 接口 |
| 跨模块调用 | Mock 对方模块的 Service 接口 |
| 外部 HTTP 调用 | `httptest.NewServer` + 录制回放 |
| 数据库 | 集成测试用真实 DB / 单元测试用 sqlmock |
| Redis | miniredis 模拟 |

---

## 集成测试

### 测试数据库

```go
// testutil/db.go
func SetupTestDB(t *testing.T) *gorm.DB {
    // 使用 dockertest 启动 PostgreSQL 容器
    pool, err := dockertest.NewPool("")
    resource, err := pool.Run("postgres", "16-alpine", []string{
        "POSTGRES_PASSWORD=test",
        "POSTGRES_DB=test",
    })

    var db *gorm.DB
    pool.Retry(func() error {
        db, err = gorm.Open(postgres.Open(
            fmt.Sprintf("host=localhost port=%s user=postgres password=test dbname=test sslmode=disable",
                resource.GetPort("5432/tcp"))))
        return err
    })

    t.Cleanup(func() { pool.Purge(resource) })
    return db
}
```

### API 集成测试

覆盖完整请求链路：

```go
func TestAuthAPI_RegisterAndLogin(t *testing.T) {
    db := SetupTestDB(t)
    router := setupTestApp(t, db)

    // 注册
    w := httptest.NewRecorder()
    router.ServeHTTP(w, req("POST", "/api/v1/auth/register", `{"email":"test@test.com","password":"Pass1234"}`))
    assert.Equal(t, 201, w.Code)

    // 登录
    w = httptest.NewRecorder()
    router.ServeHTTP(w, req("POST", "/api/v1/auth/login", `{"email":"test@test.com","password":"Pass1234"}`))
    assert.Equal(t, 200, w.Code)

    // 使用返回的 JWT 访问需鉴权的接口
    token := parseToken(t, w.Body)
    w = httptest.NewRecorder()
    req, _ := http.NewRequest("GET", "/api/v1/user/profile", nil)
    req.Header.Set("Authorization", "Bearer "+token)
    router.ServeHTTP(w, req)
    assert.Equal(t, 200, w.Code)
}
```

---

## 测试数据管理

### 固定数据（Fixture）

```go
// testutil/fixtures.go
var TestUsers = []model.User{
    {Email: "alice@test.com", Nickname: "Alice"},
    {Email: "bob@test.com",   Nickname: "Bob"},
}

func SeedUsers(t *testing.T, db *gorm.DB) {
    for _, u := range TestUsers {
        u.PasswordHash = hashPassword("test123")
        require.NoError(t, db.Create(&u).Error)
    }
}
```

### 工厂函数

```go
// testutil/factory.go
func RandomUser(t *testing.T) *model.User {
    return &model.User{
        Email:    fake.Email(),
        Nickname: fake.Name(),
    }
}
```

---

## 运行测试

### Makefile 命令

```makefile
test-unit:           # 单元测试（仅跑不带 _integration 的文件）
    go test ./... -short -count=1 -race

test-integration:    # 集成测试
    go test ./... -run Integration -count=1 -v -timeout 120s

test-all:            # 全部测试
    go test ./... -count=1 -race -timeout 180s

test-coverage:       # 覆盖率
    go test ./... -count=1 -coverprofile=coverage.out
    go tool cover -html=coverage.out -o coverage.html

test-pkg:            # 指定包测试
    go test ./internal/services/auth/... -count=1 -v
```

### 测试标签

```go
// 文件: auth_service_integration_test.go
//go:build integration

func TestAuthIntegration(t *testing.T) {
    if testing.Short() { t.Skip("跳过集成测试") }
    // ...
}
```

```bash
# 只跑单元测试
make test-unit

# 跑全部测试（含集成）
make test-all
```

---

## 覆盖率目标

| 层级 | 目标 |
|------|------|
| pkg/（工具库） | ≥ 90% |
| Service 层 | ≥ 80% |
| Handler 层 | ≥ 75% |
| Repository 层 | ≥ 70% |
| **整体** | **≥ 70%** |

```bash
# 检查覆盖率
make test-coverage
# 打开 HTML 报告
open coverage.html
```

### 覆盖重点

- **所有公开函数**必须有测试
- **错误路径**必须测试（数据库错误、权限不足、参数非法）
- **边界值**（空数据、超大值、特殊字符）
- **并发安全**（goroutine 竞态用 `-race` 检测）

---

## CI 集成

```yaml
# .github/workflows/test.yml
on: [push, pull_request]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: "1.22"

      - name: Lint
        uses: golangci/golangci-lint-action@v4

      - name: Unit Tests
        run: make test-unit

      - name: Integration Tests
        run: make test-integration

      - name: Coverage
        run: make test-coverage

      - name: Upload Coverage
        uses: codecov/codecov-action@v4
```

---

## 各模块测试清单

| 模块 | 单元测试 | 集成测试 | Mock 对象 |
|------|----------|----------|-----------|
| pkg/crypto | 加密/解密/哈希/压缩算法 | — | — |
| pkg/net | HTTP 客户端重试/超时 | 真实 HTTP server | 外部 API |
| database | — | 真实 PG/Redis 连接池 | — |
| config | Viper 加载逻辑 | 配置文件读写 | — |
| auth | 登录/注册/OAuth callback | 完整 API 链路 | User repository |
| user | 资料更新/设备绑定 | 用户 CRUD API | Auth service |
| notify | 消息渲染/渠道选择 | 邮件 SMTP / Push | 外部推送 SDK |
| file | 上传/下载逻辑 | RustFS 上传 | S3 客户端 |
| scheduler | cron 解析/任务注册 | asynq 任务执行 | Task handler |
| jobs | 进度上报/取消信号 | 长时间任务执行 | Scheduler |
| tagging | 标签 CRUD/组合查询 | 标签关联 API | — |
| analytics | 聚合查询构建 | 真实数据聚合 | Database |
| eventbus | 发布/订阅/死信 | Redis Stream | — |
| webhook | 签名/模板/重试 | HTTP 回调 | 外部服务 |
| mqtt | 消息序列化/桥接 | 真实 MQTT Broker | — |
| media | 元数据读写 | 真实音频文件 | Filesystem |
| s1 物品管理 | CRUD 业务逻辑 | 完整 API | Search/Tagging/Analytics |

---

## 常见场景测试模板

### HTTP Handler

```go
func jsonBody(t *testing.T, v any) *bytes.Reader {
    data, err := json.Marshal(v)
    require.NoError(t, err)
    return bytes.NewReader(data)
}

func assertJSON(t *testing.T, expected, actual map[string]any) {
    if diff := cmp.Diff(expected, actual); diff != "" {
        t.Errorf("JSON mismatch (-want +got):\n%s", diff)
    }
}
```

### 异步任务

```go
func TestJob_ProgressTracking(t *testing.T) {
    jc := NewJobContext("test-job")

    go func() {
        for i := 0; i < 100; i++ {
            jc.SetProgress(float64(i) / 100.0)
            time.Sleep(10 * time.Millisecond)
        }
    }()

    assert.Eventually(t, func() bool {
        return jc.Progress() >= 1.0
    }, 5*time.Second, 50*time.Millisecond)
}
```

### 并发安全

```go
func TestCache_ConcurrentAccess(t *testing.T) {
    cache := NewMemoryCache(1 * time.Minute)

    var wg sync.WaitGroup
    for i := 0; i < 100; i++ {
        wg.Add(1)
        go func(n int) {
            defer wg.Done()
            cache.Set(fmt.Sprintf("key:%d", n), n, 1*time.Minute)
        }(i)
    }
    wg.Wait()

    for i := 0; i < 100; i++ {
        val, ok := cache.Get(fmt.Sprintf("key:%d", i))
        assert.True(t, ok)
        assert.Equal(t, i, val)
    }
}
```
