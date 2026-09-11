package service

import (
	"net/url"
	"strings"
	"testing"
)

func TestCanonicalQueryEmpty(t *testing.T) {
	if got := CanonicalQuery(url.Values{}); got != "" {
		t.Fatalf("empty query = %q, want empty", got)
	}
}

func TestCanonicalQuerySortedAndEscaped(t *testing.T) {
	// 乱序、重复 key：结果必须确定性且按 key/value 字典序。
	q := url.Values{}
	q.Set("b", "2")
	q.Add("a", "1")
	q.Add("a", "0")
	if got, want := CanonicalQuery(q), "a=0&a=1&b=2"; got != want {
		t.Fatalf("CanonicalQuery = %q, want %q", got, want)
	}
	// 需转义的值按 url.QueryEscape 规则编码。
	e := url.Values{"x": []string{"a b&c=1"}}
	if got, want := CanonicalQuery(e), "x=a+b%26c%3D1"; got != want {
		t.Fatalf("CanonicalQuery escaped = %q, want %q", got, want)
	}
}

func TestCanonicalQueryMultiValueSorted(t *testing.T) {
	q := url.Values{"k": []string{"z", "m", "a"}}
	if got, want := CanonicalQuery(q), "k=a&k=m&k=z"; got != want {
		t.Fatalf("CanonicalQuery = %q, want %q", got, want)
	}
}

func TestSignVerifyRoundTrip(t *testing.T) {
	secret := "s3cret"
	sts := StringToSign("POST", "/open/v1/orders", url.Values{"a": []string{"1"}}, []byte(`{"x":1}`), "1700000000", "nonce-1")
	sig := Sign(secret, sts)
	if !Verify(secret, sts, sig) {
		t.Fatalf("verify round-trip failed")
	}
	if !Verify(secret, sts, strings.ToUpper(sig)) {
		t.Fatalf("verify should be case-insensitive for hex input")
	}
}

func TestVerifyTamperFails(t *testing.T) {
	secret := "s3cret"
	sts := StringToSign("GET", "/open/v1/products", nil, nil, "1700000000", "n")
	if Verify("wrong-secret", sts, Sign(secret, sts)) {
		t.Fatalf("verify with wrong secret must fail")
	}
	if Verify(secret, sts+" ", Sign(secret, sts)) {
		t.Fatalf("verify with tampered sts must fail")
	}
	if Verify(secret, sts, "deadbeef") {
		t.Fatalf("verify with bogus signature must fail")
	}
}

func TestStringToSignBodyDigest(t *testing.T) {
	// body 相同则签名串相同；body 变化则签名串变化。
	a := StringToSign("POST", "/p", nil, []byte(`{"a":1}`), "1", "n")
	b := StringToSign("POST", "/p", nil, []byte(`{"a":1}`), "1", "n")
	c := StringToSign("POST", "/p", nil, []byte(`{"a":2}`), "1", "n")
	if a != b {
		t.Fatalf("same inputs must produce same sts")
	}
	if a == c {
		t.Fatalf("different body must produce different sts")
	}
}
