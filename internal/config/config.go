package config

import (
	"fmt"
	"log"
	"os"
	"strings"
	"sync"

	"github.com/fsnotify/fsnotify"
	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

const (
	EnvLocal = "local"
	EnvTest  = "test"
	EnvProd  = "prod"

	configFileLocal = "config.local.yaml"
	configFileTest  = "config.test.yaml"
	configFileProd  = "config.prod.yaml"
)

// Config 应用配置根结构
type Config struct {
	Server    ServerConfig    `mapstructure:"server"`
	Log       LogConfig       `mapstructure:"log"`
	Database  DatabaseConfig  `mapstructure:"database"`
	Redis     RedisConfig     `mapstructure:"redis"`
	Cache     CacheConfig     `mapstructure:"cache"`
	Auth      AuthConfig      `mapstructure:"auth"`
	Config    ConfigOpts      `mapstructure:"config"`
	Scheduler SchedulerConfig `mapstructure:"scheduler"`
}

type ServerConfig struct {
	Name    string `mapstructure:"name"`    // 服务名称
	Version string `mapstructure:"version"` // 版本号
	Port    int    `mapstructure:"port"`    // HTTP 监听端口
	Mode    string `mapstructure:"mode"`    // Gin 运行模式
}

type LogConfig struct {
	Level      string `mapstructure:"level"`        // 日志级别
	Format     string `mapstructure:"format"`       // 日志格式(json/text)
	Output     string `mapstructure:"output"`       // 日志输出目标
	LogDir     string `mapstructure:"log_dir"`      // 日志目录（rotate 模式）
	MaxSizeMB  int    `mapstructure:"max_size_mb"`  // 单文件最大 MB
	MaxAgeDays int    `mapstructure:"max_age_days"` // 日志保留天数
}

type DatabaseConfig struct {
	MySQL    map[string]MySQLConfig `mapstructure:"mysql"`    // MySQL 实例集（主数据库）
	Postgres map[string]PGConfig    `mapstructure:"postgres"` // PostgreSQL 实例集（NAS）
	S3       map[string]S3Config    `mapstructure:"s3"`       // S3 兼容对象存储
}

type MySQLConfig struct {
	Enabled         bool   `mapstructure:"enabled"`
	Host            string `mapstructure:"host"`
	Port            int    `mapstructure:"port"`
	User            string `mapstructure:"user"`
	Password        string `mapstructure:"password"`
	DBName          string `mapstructure:"dbname"`
	MaxOpen         int    `mapstructure:"max_open"`          // 最大打开连接数
	MaxIdle         int    `mapstructure:"max_idle"`          // 最大空闲连接数
	ConnMaxLifetime string `mapstructure:"conn_max_lifetime"` // 连接最大存活时间
}

type PGConfig struct {
	Enabled         bool   `mapstructure:"enabled"`
	Host            string `mapstructure:"host"`
	Port            int    `mapstructure:"port"`
	User            string `mapstructure:"user"`
	Password        string `mapstructure:"password"`
	DBName          string `mapstructure:"dbname"`
	MaxOpen         int    `mapstructure:"max_open"`
	MaxIdle         int    `mapstructure:"max_idle"`
	ConnMaxLifetime string `mapstructure:"conn_max_lifetime"`
}

type S3Config struct {
	Enabled   bool   `mapstructure:"enabled"`
	Endpoint  string `mapstructure:"endpoint"`
	AccessKey string `mapstructure:"access_key"`
	SecretKey string `mapstructure:"secret_key"`
	UseSSL    bool   `mapstructure:"use_ssl"`
	Bucket    string `mapstructure:"bucket"`
	Region    string `mapstructure:"region"`
}

type RedisConfig struct {
	Main  RedisInstance            `mapstructure:"main"`
	Extra map[string]RedisInstance `mapstructure:"extra"` // 额外 Redis 实例
}

type RedisInstance struct {
	Enabled  bool   `mapstructure:"enabled"`
	Addr     string `mapstructure:"addr"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
	PoolSize int    `mapstructure:"pool_size"` // 连接池大小
}

type CacheConfig struct {
	DefaultTTL    string `mapstructure:"default_ttl"`     // 默认 TTL（如 5m/1h）
	DefaultNilTTL string `mapstructure:"default_nil_ttl"` // 空值缓存 TTL（如 30s）
	Jitter        string `mapstructure:"jitter"`          // TTL 随机抖动范围（如 10s/1m）
	L1MaxSize     int    `mapstructure:"l1_max_size"`     // L1 内存缓存最大条目数
	RedisKey      string `mapstructure:"redis_key"`       // 使用的 Redis 实例名称（如 main）
}

type AuthConfig struct {
	JWTSecret string       `mapstructure:"jwt_secret"` // JWT 签名密钥
	JWTExpire string       `mapstructure:"jwt_expire"` // JWT 过期时长
	OAuth2    OAuth2Config `mapstructure:"oauth2"`
}

type OAuth2Config struct {
	Wechat *OAuth2ClientConfig `mapstructure:"wechat"`
	Feishu *OAuth2ClientConfig `mapstructure:"feishu"`
	QQ     *OAuth2ClientConfig `mapstructure:"qq"`
	Apple  *AppleOAuth2Config  `mapstructure:"apple"`
	Huawei *OAuth2ClientConfig `mapstructure:"huawei"`
	Honor  *OAuth2ClientConfig `mapstructure:"honor"`
}

type OAuth2ClientConfig struct {
	ClientID     string `mapstructure:"client_id"`
	ClientSecret string `mapstructure:"client_secret"`
	RedirectURL  string `mapstructure:"redirect_url"`
}

type AppleOAuth2Config struct {
	ClientID    string `mapstructure:"client_id"`
	TeamID      string `mapstructure:"team_id"`
	KeyID       string `mapstructure:"key_id"`
	PrivateKey  string `mapstructure:"private_key"`
	RedirectURL string `mapstructure:"redirect_url"`
}

type ConfigOpts struct {
	Watch         bool   `mapstructure:"watch"`          // 启用配置热加载
	WatchInterval string `mapstructure:"watch_interval"` // 文件监听间隔
}

type SchedulerConfig struct {
	Enabled           bool `mapstructure:"enabled"`
	WorkerConcurrency int  `mapstructure:"worker_concurrency"`
	DefaultTimeout    int  `mapstructure:"default_timeout"`
	DefaultMaxRetries int  `mapstructure:"default_max_retries"`
	LogRetentionDays  int  `mapstructure:"log_retention_days"`
}

// Reloader 热加载接口，各模块实现此接口以支持配置热更新
type Reloader interface {
	Reload(cfg *Config) error
}

// Validator 配置校验接口，各模块实现此接口以注册自己的校验逻辑
type Validator interface {
	Validate(cfg *Config) error
}

var (
	mu          sync.RWMutex        // 保护 globalCfg 并发访问
	v           *viper.Viper        // Viper 实例
	globalCfg   *Config             // 全局配置缓存
	prevCfg     *Config             // 上一个有效配置（用于回滚）
	rls         []Reloader          // 已注册的热加载器列表
	validators  []Validator         // 已注册的校验器列表
	changeHooks []func(cfg *Config) // 配置变更回调（用于 EventBus 等）
)

func resolveConfigPath() string {
	env := os.Getenv("APP_ENV")
	if env == "" {
		env = EnvLocal
	}
	switch env {
	case EnvLocal:
		return configFileLocal
	case EnvTest:
		return configFileTest
	case EnvProd:
		return configFileProd
	default:
		return configFileLocal
	}
}

// Load 加载配置文件，根据 APP_ENV 自动选择环境配置
func Load(path string) (*Config, error) {
	if _, err := os.Stat(".env"); err == nil {
		if err := godotenv.Load(); err != nil {
			return nil, fmt.Errorf("加载 .env 失败: %w", err)
		}
	}

	if path == "" {
		path = resolveConfigPath()
	}

	v = viper.New()
	v.SetConfigFile(path)

	v.SetEnvPrefix("APP")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("读取配置文件失败 [%s]: %w", path, err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("解析配置失败: %w", err)
	}

	if err := Validate(&cfg); err != nil {
		return nil, fmt.Errorf("配置校验失败: %w", err)
	}

	mu.Lock()
	globalCfg = &cfg
	mu.Unlock()

	if cfg.Config.Watch {
		v.WatchConfig()
		v.OnConfigChange(func(e fsnotify.Event) {
			if err := reloadConfig(); err != nil {
				log.Printf("[config] 热加载失败: %v", err)
			}
		})
	}

	return &cfg, nil
}

func reloadConfig() error {
	var newCfg Config
	if err := v.Unmarshal(&newCfg); err != nil {
		return fmt.Errorf("解析新配置失败: %w", err)
	}
	if err := Validate(&newCfg); err != nil {
		return fmt.Errorf("新配置校验失败: %w", err)
	}

	mu.Lock()
	prevCfg = globalCfg
	globalCfg = &newCfg
	mu.Unlock()

	var errs []string
	for _, r := range rls {
		if err := r.Reload(&newCfg); err != nil {
			errs = append(errs, err.Error())
		}
	}

	if len(errs) > 0 {
		mu.Lock()
		globalCfg = prevCfg
		prevCfg = nil
		mu.Unlock()

		return fmt.Errorf("模块重载失败，已回滚: %s", strings.Join(errs, "; "))
	}

	for _, hook := range changeHooks {
		hook(&newCfg)
	}

	return nil
}

// Get 返回当前全局配置（并发安全）
func Get() *Config {
	mu.RLock()
	defer mu.RUnlock()
	return globalCfg
}

// RegisterReloader 注册配置热加载回调
func RegisterReloader(r Reloader) {
	rls = append(rls, r)
}

// RegisterValidator 注册配置校验器
func RegisterValidator(v Validator) {
	validators = append(validators, v)
}

// OnChange 注册配置变更回调（热加载成功后触发）
func OnChange(fn func(cfg *Config)) {
	changeHooks = append(changeHooks, fn)
}

// Validate 执行所有已注册的配置校验，返回聚合错误
func Validate(cfg *Config) error {
	var errs []string
	for _, v := range validators {
		if err := v.Validate(cfg); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("配置校验失败:\n%s", strings.Join(errs, "\n"))
	}
	return nil
}
