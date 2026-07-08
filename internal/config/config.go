package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/fsnotify/fsnotify"
	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

// Config 应用配置根结构
type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Log      LogConfig      `mapstructure:"log"`
	Database DatabaseConfig `mapstructure:"database"`
	Redis    RedisConfig    `mapstructure:"redis"`
	Auth     AuthConfig     `mapstructure:"auth"`
	Config   ConfigOpts     `mapstructure:"config"`
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
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
	PoolSize int    `mapstructure:"pool_size"` // 连接池大小
}

type AuthConfig struct {
	JWTSecret string `mapstructure:"jwt_secret"` // JWT 签名密钥
	JWTExpire string `mapstructure:"jwt_expire"` // JWT 过期时长
}

type ConfigOpts struct {
	Watch         bool   `mapstructure:"watch"`          // 启用配置热加载
	WatchInterval string `mapstructure:"watch_interval"` // 文件监听间隔
}

// Reloader 热加载接口，各模块实现此接口以支持配置热更新
type Reloader interface {
	Reload(cfg *Config) error
}

var (
	v         *viper.Viper // Viper 实例
	globalCfg *Config      // 全局配置缓存
	rls       []Reloader   // 已注册的热加载器列表
)

// Load 加载 .env 和配置文件，.env 中 APP_* 变量会覆盖 config.yaml
func Load(path string) (*Config, error) {
	// 加载 .env 文件（如果存在），注入环境变量供 Viper 读取
	if _, err := os.Stat(".env"); err == nil {
		if err := godotenv.Load(); err != nil {
			return nil, fmt.Errorf("加载 .env 失败: %w", err)
		}
	}

	v = viper.New()
	v.SetConfigFile(path)

	// 环境变量覆盖：APP_SERVER_PORT → server.port
	v.SetEnvPrefix("APP")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("解析配置失败: %w", err)
	}

	globalCfg = &cfg

	// 配置热加载
	if cfg.Config.Watch {
		v.WatchConfig()
		v.OnConfigChange(func(e fsnotify.Event) {
			var newCfg Config
			if err := v.Unmarshal(&newCfg); err != nil {
				return
			}
			globalCfg = &newCfg
			for _, r := range rls {
				_ = r.Reload(globalCfg)
			}
		})
	}

	return &cfg, nil
}

// Get 返回当前全局配置
func Get() *Config {
	return globalCfg
}

// RegisterReloader 注册配置热加载回调
func RegisterReloader(r Reloader) {
	rls = append(rls, r)
}
