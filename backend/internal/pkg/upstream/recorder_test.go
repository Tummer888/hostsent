package upstream

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

// memRecorder 内存接收器，用于断言采集内容。
type memRecorder struct {
	mu   sync.Mutex
	recs []Record
}

func (m *memRecorder) Record(r Record) {
	m.mu.Lock()
	m.recs = append(m.recs, r)
	m.mu.Unlock()
}

func (m *memRecorder) all() []Record {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]Record(nil), m.recs...)
}

func withRecorder(t *testing.T) *memRecorder {
	t.Helper()
	m := &memRecorder{}
	SetRecorder(m)
	SetCaptureOptions(CaptureOptions{Enabled: true, SampleRate: 100, BodyMaxBytes: 4096})
	t.Cleanup(func() {
		SetRecorder(nil)
		SetCaptureOptions(CaptureOptions{Enabled: true, SampleRate: 100, BodyMaxBytes: 4096})
	})
	return m
}

// 场景 2/3：登录请求体里的密码必须被脱敏，且摘要非空（摘要是全量原文的哈希）。
func TestCaptureMasksCredentials(t *testing.T) {
	m := withRecorder(t)
	body := []byte("username=admin&password=SuperSecret123")

	Capture(context.Background(), "POST", "https://up.example.com/v1/login?a=a",
		body, 200, []byte("eyJhbGciOi..."), nil, 12*time.Millisecond)

	recs := m.all()
	if len(recs) != 1 {
		t.Fatalf("expected 1 record, got %d", len(recs))
	}
	rec := recs[0]
	if strings.Contains(rec.RequestBody, "SuperSecret123") {
		t.Fatalf("plaintext password leaked into log: %q", rec.RequestBody)
	}
	if !strings.Contains(rec.RequestBody, "***") {
		t.Fatalf("expected masked placeholder, got %q", rec.RequestBody)
	}
	if rec.RequestBytes != len(body) {
		t.Errorf("request_bytes should be original length %d, got %d", len(body), rec.RequestBytes)
	}
	// 摘要始终基于原文计算：关掉正文采集后仍可用来比对。
	if rec.RequestDigest == "" || rec.ResponseDigest == "" {
		t.Errorf("digests must always be written, got req=%q resp=%q", rec.RequestDigest, rec.ResponseDigest)
	}
	wantDigest := digest(body)
	if rec.RequestDigest != wantDigest {
		t.Errorf("request digest should be sha256 of raw body: want %s got %s", wantDigest, rec.RequestDigest)
	}
	if !rec.Success {
		t.Error("2xx with no error should be recorded as success")
	}
	if rec.Method != "POST" || rec.Op != "" {
		t.Errorf("unexpected method/op: %s/%s", rec.Method, rec.Op)
	}
}

// 场景 4：超长请求体截断，但 request_bytes 记录原始长度。
func TestCaptureTruncatesBody(t *testing.T) {
	m := withRecorder(t)
	SetCaptureOptions(CaptureOptions{Enabled: true, SampleRate: 100, BodyMaxBytes: 128})
	big := []byte("k=" + strings.Repeat("x", 500))

	Capture(context.Background(), "POST", "https://up.example.com/x", big, 200, nil, nil, time.Millisecond)

	rec := m.all()[0]
	if !rec.RequestTruncated {
		t.Fatal("expected request_truncated=true")
	}
	if rec.RequestBytes != len(big) {
		t.Errorf("request_bytes must be the original length %d, got %d", len(big), rec.RequestBytes)
	}
	if len(rec.RequestBody) > 128 {
		t.Errorf("body should be truncated to 128 bytes, got %d", len(rec.RequestBody))
	}
}

// 场景 5：关闭采集开关后新调用不落库。
func TestCaptureDisabled(t *testing.T) {
	m := withRecorder(t)
	SetCaptureOptions(CaptureOptions{Enabled: false, SampleRate: 100, BodyMaxBytes: 4096})

	Capture(context.Background(), "GET", "https://up.example.com/x", nil, 200, []byte("{}"), nil, time.Millisecond)

	if len(m.all()) != 0 {
		t.Fatal("disabled capture must not record anything")
	}
}

// 场景 6：采样率 0 时不落库（并计入 sampled_out 计数）。
func TestCaptureSampleRateZero(t *testing.T) {
	m := withRecorder(t)
	SetCaptureOptions(CaptureOptions{Enabled: true, SampleRate: 0, BodyMaxBytes: 4096})

	Capture(context.Background(), "GET", "https://up.example.com/x", nil, 200, []byte("{}"), nil, time.Millisecond)

	if len(m.all()) != 0 {
		t.Fatal("sample rate 0 must not record anything")
	}
}

// 上游 GET 的 query 里带 token 时必须脱敏（魔方财务 ?jwt= 用法）。
func TestCaptureMasksURLQuery(t *testing.T) {
	m := withRecorder(t)

	Capture(context.Background(), "GET",
		"https://up.example.com/host/header?host_id=9&jwt=abcdef.ghijkl",
		nil, 200, []byte("{}"), nil, time.Millisecond)

	rec := m.all()[0]
	if strings.Contains(rec.URL, "abcdef.ghijkl") {
		t.Fatalf("jwt leaked into url: %q", rec.URL)
	}
	if !strings.Contains(rec.URL, "host_id=9") {
		t.Errorf("non-sensitive query params should be preserved, got %q", rec.URL)
	}
	// query 会被当作请求体参与摘要与脱敏。
	if strings.Contains(rec.RequestBody, "abcdef.ghijkl") {
		t.Fatalf("jwt leaked into request body: %q", rec.RequestBody)
	}
}

// 业务调用：一次 Trace 内的重试会产生两条记录，最后一条带业务判定结果。
func TestTraceRecordsRetriesAndBusinessOutcome(t *testing.T) {
	m := withRecorder(t)

	ctx, tr := NewTrace(context.Background(), "CreateInstance", "trace-abc")
	tr.SetProvider(func() (uint, string, string) { return 7, "魔方云A", "mofangyun" })

	// 第一次 401
	Capture(ctx, "POST", "https://up.example.com/clouds", []byte("hostname=a"), 401, []byte("unauthorized"), nil, time.Millisecond)
	// 重试成功
	Capture(ctx, "POST", "https://up.example.com/clouds", []byte("hostname=a"), 200, []byte(`{"id":42,"error":null}`), nil, time.Millisecond)
	// 业务层最终成功
	tr.Finish(nil)

	recs := m.all()
	if len(recs) != 2 {
		t.Fatalf("expected 2 records (401 + retry), got %d", len(recs))
	}
	if recs[0].StatusCode != 401 || recs[0].Success {
		t.Errorf("first attempt should be recorded as a failed 401, got %+v", recs[0])
	}
	if recs[0].RetryIndex != 0 {
		t.Errorf("first attempt retry_index should be 0, got %d", recs[0].RetryIndex)
	}
	if recs[1].StatusCode != 200 || !recs[1].Success {
		t.Errorf("second attempt should be success, got %+v", recs[1])
	}
	if recs[1].RetryIndex != 1 {
		t.Errorf("retry attempt retry_index should be 1, got %d", recs[1].RetryIndex)
	}
	if recs[0].TraceID != "trace-abc" || recs[1].Op != "CreateInstance" {
		t.Errorf("trace id / op not propagated: %+v", recs[1])
	}
	if recs[1].ProviderID != 7 || recs[1].ProviderName != "魔方云A" || recs[1].ProviderType != "mofangyun" {
		t.Errorf("provider info not attached: %+v", recs[1])
	}
}

// HTTP 2xx 但业务失败（上游用 body.error 表达）必须记 success=false 且 error 有值。
func TestTraceBusinessFailureIsNotSuccess(t *testing.T) {
	m := withRecorder(t)

	ctx, tr := NewTrace(context.Background(), "Login", "")
	Capture(ctx, "POST", "https://up.example.com/login", nil, 200, []byte(`{"error":"账号密码错误"}`), nil, time.Millisecond)
	tr.Finish(&ProviderError{Op: "Login", Code: 200, Msg: "账号密码错误"})

	rec := m.all()[0]
	if rec.Success {
		t.Fatal("business failure must not be recorded as success")
	}
	if rec.ErrorCode == "" || !strings.Contains(rec.ErrorMessage, "账号密码错误") {
		t.Errorf("error code/message must be filled, got %q / %q", rec.ErrorCode, rec.ErrorMessage)
	}
}

// 传输层错误（连不上）也要留痕，否则最难排查的情况恰好没有日志。
func TestCaptureTransportError(t *testing.T) {
	m := withRecorder(t)

	Capture(context.Background(), "POST", "https://up.example.com/x", nil, 0, nil,
		errors.New("dial tcp: connection refused"), 30*time.Millisecond)

	rec := m.all()[0]
	if rec.Success {
		t.Fatal("transport error must not be success")
	}
	if rec.StatusCode != 0 || !strings.Contains(rec.ErrorMessage, "connection refused") {
		t.Errorf("unexpected record: %+v", rec)
	}
	// 场景 2：失败的记录必须留下可聚合的错误码，即便根本没有 HTTP 响应。
	if rec.ErrorCode == "" {
		t.Errorf("failed record must carry an error_code, got %+v", rec)
	}
}

// 场景 2：非 2xx 的结构性往返（重试场景里被提前上报的那次）也要带状态码。
func TestRetryAttemptFailureCarriesStatusCode(t *testing.T) {
	m := withRecorder(t)

	ctx, tr := NewTrace(context.Background(), "CreateInstance", "")
	Capture(ctx, "POST", "https://up.example.com/clouds", nil, 401, []byte("unauthorized"), nil, time.Millisecond)
	Capture(ctx, "POST", "https://up.example.com/clouds", nil, 200, []byte(`{"id":1}`), nil, time.Millisecond)
	tr.Finish(nil)

	recs := m.all()
	if len(recs) != 2 {
		t.Fatalf("expected 2 records, got %d", len(recs))
	}
	if recs[0].Success || recs[0].ErrorCode != "401" {
		t.Errorf("failed first attempt must carry error_code=401, got %+v", recs[0])
	}
	if !recs[1].Success || recs[1].ErrorCode != "" {
		t.Errorf("successful retry must not carry an error_code, got %+v", recs[1])
	}
}
