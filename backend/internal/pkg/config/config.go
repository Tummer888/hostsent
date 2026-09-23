package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	App        AppConfig        `mapstructure:"app"`
	Auth       AuthConfig       `mapstructure:"auth"`
	Database   DatabaseConfig   `mapstructure:"database"`
	Redis      RedisConfig      `mapstructure:"redis"`
	Pricing    PricingConfig    `mapstructure:"pricing"`
	Storage    StorageConfig    `mapstructure:"storage"`
	Revalidate RevalidateConfig `mapstructure:"revalidate"`
}

// RevalidateConfig 门户主动缓存失效（doc80 §10.1）。
//
// 门户（frontend-site）的公开页面走 Nitro SWR，缓存存在 Node 进程内存里，Go 侧碰不到，
// 只能回调门户的内部接口来清。PortalURL 为空时该能力整体空转（纯后端联调环境）。
type RevalidateConfig struct {
	// PortalURL 门户基地址，如 http://127.0.0.1:3003。
	PortalURL string `mapstructure:"portal_url"`
	// Token 内部接口密钥，必须与门户的 NUXT_INTERNAL_TOKEN 一致。
	Token string `mapstructure:"token"`
}

// StorageConfig 本地文件存储配置（工单附件用，S2）。
type StorageConfig struct {
	// Root 存储根目录；相对路径按进程工作目录解析。
	Root string `mapstructure:"root"`
}

// PricingConfig 统一算价管线配置（P5-03）。
type PricingConfig struct {
	// StackMode 多来源折扣叠加模式：best（取最优，默认）/ stack（按顺序叠加）。
	StackMode string `mapstructure:"stack_mode"`
}

type RedisConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
	// Required 为 true 时 Redis 连不上将直接拒绝启动；默认 false，
	// 即 Redis 视为「加速器」而非依赖，连不通时全链路降级（doc89 §3.3）。
	Required bool `mapstructure:"required"`
}

type AppConfig struct {
	Name         string `mapstructure:"name"`
	Env          string `mapstructure:"env"`
	Host         string `mapstructure:"host"`
	Port         int    `mapstructure:"port"`
	ReadTimeout  int    `mapstructure:"read_timeout"`
	WriteTimeout int    `mapstructure:"write_timeout"`
	EncryptKey   string `mapstructure:"encrypt_key"`
	// TrustedProxies 可信反向代理网段（CIDR 或 IP），逗号分隔。
	//
	// 决定 gin 是否采信 X-Forwarded-For / X-Real-IP：只有直连对端落在这里面时，
	// 这些头部才会被用来还原客户端 IP，否则一律忽略并记对端地址。
	//
	// 默认 127.0.0.1/8 + ::1：开发态前端 dev server 直连本机后端，需要它才认
	// vite 转发过来的 X-Forwarded-For（见各 vite.config.ts 的 xfwd）。
	// 生产部署必须显式追加实际入口（宿主机 nginx / 云负载均衡网段），否则
	// 用户 IP 会退化成「代理的 IP」；反之若把 0.0.0.0/0 写进来，
	// 任意调用方都能用 X-Forwarded-For 伪造自己的 IP。
	TrustedProxies []string `mapstructure:"trusted_proxies"`
}

type AuthConfig struct {
	BearerPrefix   string `mapstructure:"bearer_prefix"`
	JWTSecret      string `mapstructure:"jwt_secret"`
	JWTIssuer      string `mapstructure:"jwt_issuer"`
	JWTExpireHours int    `mapstructure:"jwt_expire_hours"`
}

type DatabaseConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	Name     string `mapstructure:"name"`
	SSLMode  string `mapstructure:"sslmode"`
}

func Load() (*Config, error) {
	v := viper.New()
	v.SetConfigType("yaml")
	v.SetConfigName("config")
	v.AddConfigPath("./configs")
	v.AddConfigPath("../configs")
	v.AddConfigPath("../../configs")

	v.SetEnvPrefix("HOSTSENT")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	setDefaults(v)

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("read config: %w", err)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	// trusted_proxies 用 []string 接，而 viper 的 AutomaticEnv 对**切片**不做自动
	// 绑定：HOSTSENT_APP_TRUSTED_PROXIES 只在 Unmarshal 时才会被读成
	// `[a b c]` 这类字符串切片字面量，写 "10.1.2.0/24,10.1.3.0/24" 会 unmarshal
	// 失败或落成单个元素。容器化部署靠环境变量注入，这里显式解析一遍，
	// 两种写法（逗号分隔 / YAML 字面量）都认。
	if raw := strings.TrimSpace(v.GetString("app.trusted_proxies")); raw != "" {
		if proxies := parseTrustedProxies(raw); len(proxies) > 0 {
			cfg.App.TrustedProxies = proxies
		}
	}

	return &cfg, nil
}

// parseTrustedProxies 解析可信代理配置，兼容三种写法：
//   - YAML 字面量切片：`[127.0.0.1/8, ::1]`（viper 读环境变量时也会长这样）
//   - 逗号/分号分隔：`10.1.2.0/24,10.1.3.0/24`
//   - 空格分隔：`10.1.2.0/24 10.1.3.0/24`
func parseTrustedProxies(raw string) []string {
	trimmed := strings.Trim(strings.TrimSpace(raw), "[]")
	fields := strings.FieldsFunc(trimmed, func(r rune) bool {
		return r == ',' || r == ';' || r == ' ' || r == '\t' || r == '\n'
	})
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		if f = strings.Trim(strings.TrimSpace(f), `"'`); f != "" {
			out = append(out, f)
		}
	}
	return out
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("app.name", "hostsent-backend")
	v.SetDefault("app.env", "development")
	v.SetDefault("app.host", "0.0.0.0")
	v.SetDefault("app.port", 8080)
	v.SetDefault("app.read_timeout", 10)
	v.SetDefault("app.write_timeout", 10)
	v.SetDefault("app.encrypt_key", "hostsent-encrypt-key")
	// 可信代理默认只含回环：开发态 vite dev server 直连本机后端要认 X-Forwarded-For。
	// 默认值刻意不含 docker 网段/私网全段 —— 默认必须是「不信任任何转发头」的保守
	// 口径，否则任何能直连后端的人都能用 X-Forwarded-For 伪造 IP（该值同时是登录
	// 失败锁定的键）。容器/生产环境由 configs/config.yaml 或
	// HOSTSENT_APP_TRUSTED_PROXIES 显式声明真实入口。
	v.SetDefault("app.trusted_proxies", []string{"127.0.0.1/8", "::1"})
	v.SetDefault("auth.bearer_prefix", "Bearer")
	v.SetDefault("auth.jwt_secret", "hostsent-dev-secret")
	v.SetDefault("auth.jwt_issuer", "hostsent-backend")
	v.SetDefault("auth.jwt_expire_hours", 24)
	v.SetDefault("database.host", "postgres")
	v.SetDefault("database.port", 5432)
	v.SetDefault("database.user", "hostsent")
	v.SetDefault("database.password", "hostsent")
	v.SetDefault("database.name", "hostsent")
	v.SetDefault("database.sslmode", "disable")
	v.SetDefault("redis.host", "redis")
	v.SetDefault("redis.port", 6379)
	v.SetDefault("redis.password", "")
	v.SetDefault("redis.db", 0)
	// 默认不强制 Redis：连不通时降级运行，业务侧走 DB 兜底（doc91 §2.2）。
	v.SetDefault("redis.required", false)
	// 算价管线默认取最优折扣（P5-03）。
	v.SetDefault("pricing.stack_mode", "best")
	// 附件落盘目录（相对工作目录）；生产用只读根镜像时通过 HOSTSENT_STORAGE_ROOT 覆盖为挂载卷。
	v.SetDefault("storage.root", "./uploads")
	// 门户主动缓存失效：默认不配地址（不尝试回调任何外部服务），部署时注入。
	v.SetDefault("revalidate.portal_url", "")
	v.SetDefault("revalidate.token", "")
}
