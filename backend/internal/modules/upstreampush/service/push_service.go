// Package service 实现上游推送的验签与业务落地。
//
// 威胁模型（为什么这层的每一行都要看）：
//
//	/api/host/sync 免登录，能改实例状态、到期时间与 IP。若不验签，任何人构造
//	一个 POST 就能把某台机器的到期时间改到下世纪、或把状态改成"运行中"。
//	唯一的凭据是 32 位 downstream_token，它只在开通握手时由我方下发给上游，
//	存在 orders.downstream_token。验签必须与上游 createSign 逐字节一致。
package service

import (
	"context"
	"crypto/md5"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"

	"hostsent/backend/internal/modules/upstreampush/dto"
	"hostsent/backend/internal/modules/upstreampush/repository"
)

// ErrSignature 验签失败。对外统一返回"签名验证失败"，不区分是令牌查不到还是
// 摘要不匹配——把两种失败区分开会让攻击者能枚举出哪些 order_id 有下游令牌。

// ErrNotFound 推送的目标不存在（订单/实例/工单查不到）。
var ErrNotFound = repository.ErrNotFound

// Notifier 通知能力（窄接口）：上游把工单回复推过来后要提醒用户。
type Notifier interface {
	Publish(ctx context.Context, event, target string, recipientID uint64, vars map[string]string, sourceID string) error
}

type noopNotifier struct{}

func (noopNotifier) Publish(context.Context, string, string, uint64, map[string]string, string) error {
	return nil
}

// Service 上游推送处理能力。
type Service interface {
	HandleHostSync(ctx context.Context, req *dto.HostSyncRequest) (*dto.HostSyncResult, error)
	HandleTicketReplySync(ctx context.Context, req *dto.TicketReplySyncRequest) (*dto.TicketReplySyncResult, error)
	// SetNotifier 延迟注入通知能力：通知服务在装配期晚于推送服务创建。
	SetNotifier(n Notifier)
}

type service struct {
	repo     repository.Repository
	notifier Notifier
	logger   *zap.Logger
	// now 便于测试注入固定时间（到期时间换算与时间戳比较都依赖它）。
	now func() time.Time
}

// NewService 创建上游推送处理服务。notifier 为 nil 时静默跳过通知。
func NewService(repo repository.Repository, notifier Notifier, logger *zap.Logger) Service {
	if notifier == nil {
		notifier = noopNotifier{}
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	return &service{repo: repo, notifier: notifier, logger: logger, now: time.Now}
}

// SetNotifier 延迟注入通知能力（通知服务在装配期晚于本服务创建）。nil 表示保持当前。
func (s *service) SetNotifier(n Notifier) {
	if n != nil {
		s.notifier = n
	}
}

// ---------------------------------------------------------------------------
// 验签
// ---------------------------------------------------------------------------

// SignPayload 复刻上游 createSign 的被签内容：把 id 与 token、rand_str 三个参数
// 按 key 字符串升序（ksort SORT_STRING）后 json_encode。
//
// 字段顺序之所以固定为 id → rand_str → token，是因为 PHP 字符串比较下
// "id" < "rand_str" < "token"。这里直接用 sort.Strings 而不是手写顺序，
// 避免以后新增参与签名的字段时改错位置。
//
// 与 PHP json_encode 的差异：PHP 默认把 "/" 转义成 "\/"、把非 ASCII 转成 \uXXXX。
// 本协议参与签名的三个值分别是数字 id、字母数字 rand_str、十六进制 token，
// 都不会命中这两条规则，故 Go 的 json.Marshal 输出与 PHP 逐字节一致
// （已用 PHP 7.3 生成基线向量做单测比对）。
func signString(id, token, randStr string) (string, error) {
	params := map[string]string{
		"id":       id,
		"token":    token,
		"rand_str": randStr,
	}
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	b.WriteByte('{')
	for i, k := range keys {
		if i > 0 {
			b.WriteByte(',')
		}
		kb, err := json.Marshal(k)
		if err != nil {
			return "", err
		}
		vb, err := json.Marshal(params[k])
		if err != nil {
			return "", err
		}
		b.Write(kb)
		b.WriteByte(':')
		b.Write(vb)
	}
	b.WriteByte('}')
	return b.String(), nil
}

// Verify 校验签名。expected 是本地存的 32 位令牌。
//
// 比较用 subtle.ConstantTimeCompare：逐字节短路比较会因"前几位就对"而变快，
// 理论上可被计时侧信道逐位爆破签名。签名是 MD5 十六进制大写串（32 字符）。
func Verify(id, randStr, signature, token string) bool {
	if id == "" || randStr == "" || signature == "" || token == "" {
		return false
	}
	payload, err := signString(id, token, randStr)
	if err != nil {
		return false
	}
	sum := md5.Sum([]byte(payload))
	want := strings.ToUpper(hex.EncodeToString(sum[:]))
	got := strings.ToUpper(strings.TrimSpace(signature))
	return subtle.ConstantTimeCompare([]byte(want), []byte(got)) == 1
}

// ---------------------------------------------------------------------------
// 主机信息推送
// ---------------------------------------------------------------------------

// 上游 domainstatus → 本地实例状态映射。
//
// 与 zjmf 适配器拉取侧的映射（provider.go:1624）保持同一口径，避免"推送来的"
// 和"定时同步拉来的"同一台机器显示成两种状态。
// Suspended/Cancelled/Deleted 都映射到 stopped/deleted 而非三个本地状态——
// 本地状态空间只有那 7 个，暂停/取消的细分由 lifecycle_stage 与 raw_data 承载。
var domainStatusToLocal = map[string]string{
	"Active":    "running",
	"Pending":   "creating",
	"Suspended": "stopped",
	"Cancelled": "stopped",
	"Fraud":     "stopped",
	"Deleted":   "deleted",
}

func (s *service) HandleHostSync(ctx context.Context, req *dto.HostSyncRequest) (*dto.HostSyncResult, error) {
	if req == nil {
		return nil, fmt.Errorf("空请求")
	}
	orderID, err := strconv.ParseUint(strings.TrimSpace(req.ID), 10, 64)
	if err != nil || orderID == 0 {
		return nil, fmt.Errorf("id 非法")
	}

	token, err := s.repo.DownstreamTokenByOrderID(ctx, orderID)
	if err != nil {
		return nil, ErrSignature
	}
	if !Verify(req.ID, req.RandStr, req.Signature, token) {
		// 验签失败要留痕：这可能是配置错位（我方换过令牌），也可能是伪造请求。
		s.logger.Warn("upstream push 验签失败", zap.Uint64("order_id", orderID))
		return nil, ErrSignature
	}

	snap, err := s.repo.InstanceByOrderID(ctx, orderID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			// 推送早于我方实例落库（上游异步开通很快）：不是错误，回 200 让上游别重试，
			// 本地随后由同步拉取补齐。若回非 200，上游会重试 5 次后放弃。
			s.logger.Info("upstream push 到达时本地尚无实例行，忽略", zap.Uint64("order_id", orderID))
			return &dto.HostSyncResult{Applied: false}, nil
		}
		return nil, err
	}

	patch, changed := s.buildInstancePatch(snap, req)
	result := &dto.HostSyncResult{InstanceID: snap.ID, ChangedFields: changed, Applied: len(changed) > 0}
	if len(changed) == 0 {
		// 内容与本地一致：不动库（避免 updated_at 被无意义刷动）。
		s.advanceOrderIfNeeded(ctx, orderID, req)
		return result, nil
	}
	patch.TouchSyncedAt = true
	if err := s.repo.ApplyInstancePatch(ctx, snap.ID, patch); err != nil {
		return nil, err
	}
	s.advanceOrderIfNeeded(ctx, orderID, req)
	s.logger.Info("upstream push 已回写实例",
		zap.Uint64("order_id", orderID), zap.Uint64("instance_id", snap.ID),
		zap.Strings("fields", changed), zap.String("domainstatus", req.DomainStatus))
	return result, nil
}

// buildInstancePatch 把推送内容换算成待写字段，并只保留"确实变了"的字段。
//
// 终态保护：本地已判 destroyed 的实例，上游软删残留仍可能推 running，
// 照抄会让已销毁机器重新显示成运行中（与 sync_engine 的同类保护一致）。
func (s *service) buildInstancePatch(snap *repository.InstanceSnapshot, req *dto.HostSyncRequest) (repository.InstancePatch, []string) {
	patch := repository.InstancePatch{}
	var changed []string

	if snap.LifecycleStage != "destroyed" {
		if target, ok := domainStatusToLocal[strings.TrimSpace(req.DomainStatus)]; ok && target != snap.Status {
			patch.Status = target
			changed = append(changed, "status")
		} else {
			patch.Status = snap.Status
		}
	} else {
		patch.Status = snap.Status
	}

	if v := strings.TrimSpace(req.DedicatedIP); v != "" && v != snap.PublicIP {
		patch.PublicIP = v
		changed = append(changed, "public_ip")
	} else {
		patch.PublicIP = snap.PublicIP
	}
	if v := strings.TrimSpace(req.AssignedIPs); v != "" && v != snap.PrivateIP {
		// assignedips 可能是逗号分隔的多 IP；本地只有一列，取首个（与 zjmf 拉取侧口径一致）。
		patch.PrivateIP = firstIP(v)
		changed = append(changed, "private_ip")
	} else {
		patch.PrivateIP = snap.PrivateIP
	}
	if v := strings.TrimSpace(req.OS); v != "" && v != snap.OS {
		patch.OS = v
		changed = append(changed, "os")
	} else {
		patch.OS = snap.OS
	}
	if expire := parseNextDue(req.NextDueDate); expire != nil {
		if snap.ExpireAt == nil || !expire.Equal(*snap.ExpireAt) {
			patch.ExpireAt = expire
			changed = append(changed, "expire_at")
		}
	}
	// 上游主机 id 落 provider_instance_id：占位行（为空）首次收到推送即完成"发现真实实例号"。
	if v := strings.TrimSpace(req.HostID); v != "" && v != snap.ProviderInstanceID {
		patch.ProviderInstanceID = v
		changed = append(changed, "provider_instance_id")
	}

	// raw_data 合并推送字段：对账（reconcile）读 raw_data.domainstatus / nextduedate，
	// 不写回去账期与状态的比对就会一直用旧值。合并而非覆盖，保留拉取侧写入的其余字段。
	rawPatch := map[string]any{
		"domainstatus": strings.TrimSpace(req.DomainStatus),
		"dedicatedip":  strings.TrimSpace(req.DedicatedIP),
		"assignedips":  strings.TrimSpace(req.AssignedIPs),
		"os":           strings.TrimSpace(req.OS),
		"port":         strings.TrimSpace(req.Port),
		"host_id":      strings.TrimSpace(req.HostID),
		"push_time":    s.now().UTC().Format(time.RFC3339),
	}
	// nextduedate 按数值写入（与拉取侧口径一致：reconcile SQL 按量级判断秒/毫秒）。
	if ts, ok := normalizeUnix(req.NextDueDate); ok {
		rawPatch["nextduedate"] = ts
	}
	merged := mergeRawData(snap.RawData, rawPatch)
	if merged != snap.RawData {
		patch.RawData = merged
		changed = append(changed, "raw_data")
	} else {
		patch.RawData = snap.RawData
	}
	return patch, changed
}

// advanceOrderIfNeeded 上游推送 type=create 且主机已 Active 时，把我方订单
// provisioning → active 补齐（异步开通的最后一段回路）。
// 只认这一条迁移；失败只记日志，不影响推送本身的 200 响应。
func (s *service) advanceOrderIfNeeded(ctx context.Context, orderID uint64, req *dto.HostSyncRequest) {
	if !strings.EqualFold(strings.TrimSpace(req.Type), "create") {
		return
	}
	if !strings.EqualFold(strings.TrimSpace(req.DomainStatus), "Active") {
		return
	}
	ok, err := s.repo.AdvanceOrderToActive(ctx, orderID)
	if err != nil {
		s.logger.Warn("upstream push 推进订单状态失败", zap.Uint64("order_id", orderID), zap.Error(err))
		return
	}
	if ok {
		s.logger.Info("upstream push 推进订单 provisioning→active", zap.Uint64("order_id", orderID))
	}
}

// ---------------------------------------------------------------------------
// 工单回复推送
// ---------------------------------------------------------------------------

func (s *service) HandleTicketReplySync(ctx context.Context, req *dto.TicketReplySyncRequest) (*dto.TicketReplySyncResult, error) {
	if req == nil {
		return nil, fmt.Errorf("空请求")
	}
	content := strings.TrimSpace(req.Content)
	if content == "" {
		return nil, fmt.Errorf("回复内容为空")
	}
	orderID, err := strconv.ParseUint(strings.TrimSpace(req.ID), 10, 64)
	if err != nil || orderID == 0 {
		return nil, fmt.Errorf("id 非法")
	}
	tid := strings.TrimSpace(req.TID)
	if tid == "" {
		return nil, fmt.Errorf("tid 非法")
	}

	token, err := s.repo.DownstreamTokenByOrderID(ctx, orderID)
	if err != nil {
		return nil, ErrSignature
	}
	if !Verify(req.ID, req.RandStr, req.Signature, token) {
		s.logger.Warn("upstream ticket push 验签失败", zap.Uint64("order_id", orderID))
		return nil, ErrSignature
	}

	ticketID, err := s.repo.TicketIDByUpstreamTID(ctx, tid)
	if err != nil {
		return nil, ErrNotFound
	}

	// 上游未带 rid 时退化为按内容去重：用 tid + 内容摘要拼一个稳定的伪回复号，
	// 这样上游重试（内容相同）仍能命中唯一索引，不会重复入库。
	replyKey := strings.TrimSpace(req.RID)
	if replyKey == "" {
		sum := md5.Sum([]byte(tid + "\x00" + content))
		replyKey = "gen:" + hex.EncodeToString(sum[:])
	} else {
		replyKey = "zjmf:" + replyKey
	}

	replyID, applied, err := s.repo.InsertUpstreamReply(ctx, ticketID, replyKey, content)
	if err != nil {
		return nil, err
	}
	if applied {
		if err := s.repo.MarkTicketWaitingUser(ctx, ticketID); err != nil {
			s.logger.Warn("upstream ticket push 推进工单状态失败", zap.Uint64("ticket_id", ticketID), zap.Error(err))
		}
		userID, ticketNo, err := s.repo.TicketUserAndNo(ctx, ticketID)
		if err == nil && userID > 0 {
			// 幂等键带 replyID：同一条上游回复只通知一次。
			_ = s.notifier.Publish(ctx, "ticket_replied", "user", userID,
				map[string]string{"ticket_no": ticketNo},
				fmt.Sprintf("ticket:%d:upstream-reply:%d", ticketID, replyID))
		}
		s.logger.Info("upstream ticket push 已落库",
			zap.Uint64("ticket_id", ticketID), zap.Uint64("reply_id", replyID), zap.String("tid", tid))
	}
	return &dto.TicketReplySyncResult{Applied: applied, TicketID: ticketID, ReplyID: replyID}, nil
}

// ---------------------------------------------------------------------------
// 小工具
// ---------------------------------------------------------------------------

// firstIP 取逗号/换行分隔 IP 串的最后一个非空项。
//
// 取最后一个而不是第一个：上游 assignedips 的约定是"附加 IP"，主 IP 走 dedicatedip；
// 两侧都取首个会把主 IP 重复写进 private_ip。取末项至少落在真正附加的地址上。
func firstIP(s string) string {
	parts := strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == '\n' || r == ';' })
	out := ""
	for _, p := range parts {
		if v := strings.TrimSpace(p); v != "" {
			out = v
		}
	}
	return out
}

// normalizeUnix 归一 Unix 时间戳字符串，兼容秒级（10 位）与毫秒级（13 位）。
// 上游抛异常时会给空/非数字，这里返回 ok=false 让调用方跳过该字段。
func normalizeUnix(raw string) (int64, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, false
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || n <= 0 {
		return 0, false
	}
	if n > 100000000000 { // 毫秒
		n = n / 1000
	}
	return n, true
}

// parseNextDue 把上游 nextduedate 转成 *time.Time；空/非法返回 nil（表示不改）。
func parseNextDue(raw string) *time.Time {
	sec, ok := normalizeUnix(raw)
	if !ok {
		return nil
	}
	t := time.Unix(sec, 0).UTC()
	return &t
}

// mergeRawData 把 patch 里的非空值并入既有 JSON 文本，返回新的 JSON 文本。
// 既有内容非 JSON 时按空对象处理（脏数据不该让整条推送失败）。
func mergeRawData(existing string, patch map[string]any) string {
	merged := map[string]any{}
	if raw := strings.TrimSpace(existing); raw != "" {
		_ = json.Unmarshal([]byte(raw), &merged)
	}
	changed := false
	for k, v := range patch {
		if s, ok := v.(string); ok && s == "" {
			continue // 空值不覆盖既有字段（上游本次没带的字段不该被清空）
		}
		if cur, ok := merged[k]; !ok || fmt.Sprint(cur) != fmt.Sprint(v) {
			merged[k] = v
			changed = true
		}
	}
	if !changed {
		return existing
	}
	out, err := json.Marshal(merged)
	if err != nil {
		return existing
	}
	return string(out)
}
