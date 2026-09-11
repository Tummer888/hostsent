// openapp — 开放平台应用管理 CLI（P6/T6.1）。
//
// 用途：open_apps 的运营管理（建应用/启停/重置密钥/回调设置/IP 白名单），
// 密钥经 internal/pkg/crypto 以 cfg.App.EncryptKey 加密后落库，明文只在创建/重置时打印一次。
//
// 用法（在 backend/ 目录下运行，读取 configs/config.yaml）：
//
//	go run ./cmd/openapp create -name "下游平台A" -owner-user-id 100 -scopes "catalog:read,order:create"
//	go run ./cmd/openapp list
//	go run ./cmd/openapp show -app-id app_xxx
//	go run ./cmd/openapp enable -app-id app_xxx
//	go run ./cmd/openapp disable -app-id app_xxx
//	go run ./cmd/openapp reset-secret -app-id app_xxx
//	go run ./cmd/openapp set-notify -app-id app_xxx -url https://downstream/notify
//	go run ./cmd/openapp add-ip -app-id app_xxx -cidr 10.0.0.0/8
//	go run ./cmd/openapp clear-ip -app-id app_xxx
package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	openmodel "hostsent/backend/internal/modules/open/model"
	openrepo "hostsent/backend/internal/modules/open/repository"
	"hostsent/backend/internal/pkg/config"
	"hostsent/backend/internal/pkg/crypto"
)

// knownScopes 能力位允许清单（D2：不含 instance:destroy）。
var knownScopes = map[string]struct{}{
	"catalog:read":     {},
	"order:create":     {},
	"instance:read":    {},
	"instance:renew":   {},
	"instance:power":   {},
	"instance:suspend": {},
	"audit:read":       {},
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	db, err := gorm.Open(postgres.Open(dsn(cfg)), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		log.Fatalf("connect db: %v", err)
	}
	repo := openrepo.NewAppRepository(db)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := dispatch(ctx, repo, cfg, os.Args[1], os.Args[2:]); err != nil {
		log.Fatalf("openapp %s: %v", os.Args[1], err)
	}
}

func dsn(cfg *config.Config) string {
	d := cfg.Database
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		d.Host, d.Port, d.User, d.Password, d.Name, d.SSLMode)
}

func usage() {
	fmt.Fprintln(os.Stderr, "用法: openapp <create|list|show|enable|disable|reset-secret|set-notify|add-ip|clear-ip> [flags]")
}

func dispatch(ctx context.Context, repo openrepo.AppRepository, cfg *config.Config, cmd string, args []string) error {
	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
	name := fs.String("name", "", "应用名称")
	owner := fs.Uint64("owner-user-id", 0, "归属平台账号 user_id（定价/余额/实例归属）")
	scopes := fs.String("scopes", "catalog:read", "逗号分隔能力位")
	rateLimit := fs.Int("rate-limit", 0, "限速（次/分钟，0=默认）")
	notifyURL := fs.String("url", "", "事件回调地址")
	appID := fs.String("app-id", "", "应用 app_id")
	cidrs := fs.String("cidr", "", "逗号分隔 CIDR 白名单")
	_ = fs.Parse(args)

	switch cmd {
	case "create":
		return cmdCreate(ctx, repo, cfg, *name, *owner, *scopes, *rateLimit, *notifyURL)
	case "list":
		return cmdList(ctx, repo)
	case "show":
		return cmdShow(ctx, repo, cfg, *appID)
	case "enable":
		return repo.UpdateStatus(ctx, *appID, openmodel.OpenAppStatusEnabled)
	case "disable":
		return repo.UpdateStatus(ctx, *appID, openmodel.OpenAppStatusDisabled)
	case "reset-secret":
		return cmdResetSecret(ctx, repo, cfg, *appID)
	case "set-notify":
		return cmdSetNotify(ctx, repo, cfg, *appID, *notifyURL)
	case "add-ip":
		if strings.TrimSpace(*cidrs) == "" {
			return fmt.Errorf("-cidr 必填")
		}
		return repo.AddIPRules(ctx, *appID, splitAndTrim(*cidrs))
	case "clear-ip":
		return repo.DeleteIPRules(ctx, *appID)
	default:
		usage()
		return fmt.Errorf("未知子命令 %q", cmd)
	}
}

func cmdCreate(ctx context.Context, repo openrepo.AppRepository, cfg *config.Config, name string, owner uint64, scopes string, rateLimit int, notifyURL string) error {
	if owner == 0 {
		return fmt.Errorf("-owner-user-id 必填：定价与订单归属挂该账号（D3）")
	}
	list, err := parseScopes(scopes)
	if err != nil {
		return err
	}
	appID, err := randomToken("app", 8)
	if err != nil {
		return err
	}
	secret, err := randomSecret()
	if err != nil {
		return err
	}
	notifySecret, err := randomSecret()
	if err != nil {
		return err
	}
	encSecret, err := crypto.Encrypt(secret, cfg.App.EncryptKey)
	if err != nil {
		return err
	}
	encNotify, err := crypto.Encrypt(notifySecret, cfg.App.EncryptKey)
	if err != nil {
		return err
	}
	row := &openmodel.OpenApp{
		AppID: appID, AppSecret: encSecret, Name: name,
		Status: openmodel.OpenAppStatusEnabled, RateLimit: rateLimit,
		NotifyURL: notifyURL, NotifySecret: encNotify, OwnerUserID: owner,
	}
	if err := repo.CreateApp(ctx, row, list); err != nil {
		return err
	}
	fmt.Printf("应用已创建（明文密钥仅此一次打印，请妥善保存）\n")
	fmt.Printf("  app_id:        %s\n", appID)
	fmt.Printf("  app_secret:    %s\n", secret)
	fmt.Printf("  notify_secret: %s\n", notifySecret)
	fmt.Printf("  owner_user_id: %d\n", owner)
	fmt.Printf("  scopes:        %s\n", strings.Join(list, ","))
	return nil
}

func cmdList(ctx context.Context, repo openrepo.AppRepository) error {
	apps, err := repo.ListApps(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("%-6s %-24s %-20s %-8s %-12s %s\n", "ID", "APP_ID", "NAME", "STATUS", "RATE/MIN", "OWNER")
	for _, a := range apps {
		status := "disabled"
		if a.Status == openmodel.OpenAppStatusEnabled {
			status = "enabled"
		}
		fmt.Printf("%-6d %-24s %-20s %-8s %-12d %d\n", a.ID, a.AppID, a.Name, status, a.RateLimit, a.OwnerUserID)
	}
	return nil
}

func cmdShow(ctx context.Context, repo openrepo.AppRepository, cfg *config.Config, appID string) error {
	app, err := repo.GetByAppID(ctx, appID)
	if err != nil {
		return err
	}
	secret, err := crypto.Decrypt(app.AppSecret, cfg.App.EncryptKey)
	if err != nil {
		return fmt.Errorf("解密 app_secret 失败（检查 encrypt_key 是否与建号时一致）: %w", err)
	}
	notifySecret := ""
	if app.NotifySecret != "" {
		if s, err := crypto.Decrypt(app.NotifySecret, cfg.App.EncryptKey); err == nil {
			notifySecret = s
		}
	}
	status := "disabled"
	if app.Status == openmodel.OpenAppStatusEnabled {
		status = "enabled"
	}
	fmt.Printf("app_id:         %s\nname:           %s\nstatus:         %s\nowner_user_id:  %d\nrate_limit:     %d/min\nnotify_url:     %s\napp_secret:     %s\nnotify_secret:  %s\n",
		app.AppID, app.Name, status, app.OwnerUserID, app.RateLimit, app.NotifyURL, secret, notifySecret)
	rules, err := repo.ListIPRules(ctx, appID)
	if err != nil {
		return err
	}
	if len(rules) > 0 {
		cidrs := make([]string, 0, len(rules))
		for _, r := range rules {
			cidrs = append(cidrs, r.CIDR)
		}
		fmt.Printf("ip_rules:       %s\n", strings.Join(cidrs, ","))
	}
	return nil
}

func cmdResetSecret(ctx context.Context, repo openrepo.AppRepository, cfg *config.Config, appID string) error {
	secret, err := randomSecret()
	if err != nil {
		return err
	}
	enc, err := crypto.Encrypt(secret, cfg.App.EncryptKey)
	if err != nil {
		return err
	}
	if err := repo.RotateSecret(ctx, appID, enc); err != nil {
		return err
	}
	fmt.Printf("app_secret 已重置（明文仅此一次打印）: %s\n", secret)
	return nil
}

func cmdSetNotify(ctx context.Context, repo openrepo.AppRepository, cfg *config.Config, appID, notifyURL string) error {
	if notifyURL == "" {
		return fmt.Errorf("-url 必填")
	}
	secret, err := randomSecret()
	if err != nil {
		return err
	}
	enc, err := crypto.Encrypt(secret, cfg.App.EncryptKey)
	if err != nil {
		return err
	}
	if err := repo.SetNotify(ctx, appID, notifyURL, enc); err != nil {
		return err
	}
	fmt.Printf("notify_url 已更新，notify_secret 已重置（明文仅此一次打印）: %s\n", secret)
	return nil
}

func parseScopes(raw string) ([]string, error) {
	list := splitAndTrim(raw)
	if len(list) == 0 {
		return nil, fmt.Errorf("-scopes 不能为空")
	}
	for _, s := range list {
		if _, ok := knownScopes[s]; !ok {
			return nil, fmt.Errorf("未知能力位 %q（允许值: %s）", s, strings.Join(scopeNames(), ","))
		}
	}
	return list, nil
}

func scopeNames() []string {
	names := make([]string, 0, len(knownScopes))
	for k := range knownScopes {
		names = append(names, k)
	}
	// map 遍历无序，排序仅为了输出可读。
	for i := 0; i < len(names); i++ {
		for j := i + 1; j < len(names); j++ {
			if names[j] < names[i] {
				names[i], names[j] = names[j], names[i]
			}
		}
	}
	return names
}

func splitAndTrim(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// randomToken 生成 "prefix"+16hex 的标识。
func randomToken(prefix string, bytesLen int) (string, error) {
	b := make([]byte, bytesLen)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return prefix + "_" + hex.EncodeToString(b), nil
}

// randomSecret 生成 32 字节 URL 安全随机串。
func randomSecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
