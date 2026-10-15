package service

import (
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

// 商品编码唯一约束冲突的翻译门禁。
//
// 背景（实测故障）：在「新建产品」页用了一个已存在的编码（前端按名称自动生成的
// ch-4-c-6g，与既有商品 206 撞），后端把
//   ERROR: duplicate key value violates unique constraint "idx_catalog_products_code" (SQLSTATE 23505)
// 原样返回，接口错误码 50001，前端直接把这行 SQL 显示给运营。
//
// 这些用例锁住两点：
//  1. 编码冲突必须翻译成可读中文并携带 ErrProductCodeExists（handler 据此给 20001）；
//  2. 其它唯一约束冲突（如 SKU 编码）不能被误报成商品编码问题。

func TestTranslateProductCodeConflict(t *testing.T) {
	t.Run("PG 错误且约束名匹配", func(t *testing.T) {
		err := translateProductCodeConflict(&pgconn.PgError{
			Code:           "23505",
			ConstraintName: "idx_catalog_products_code",
			Message:        "duplicate key value violates unique constraint \"idx_catalog_products_code\"",
		}, "ch-4-c-6g")

		if !errors.Is(err, ErrProductCodeExists) {
			t.Fatalf("应可归为 ErrProductCodeExists，得到 %v", err)
		}
		msg := err.Error()
		if !strings.Contains(msg, "ch-4-c-6g") {
			t.Errorf("提示应带上冲突的编码，得到：%s", msg)
		}
		if strings.Contains(msg, "SQLSTATE") || strings.Contains(msg, "constraint") {
			t.Errorf("提示不应泄漏 SQL 原文，得到：%s", msg)
		}
	})

	// GORM 有时包一层，只剩文本：仍然要能识别（否则会退回 SQL 原文）。
	t.Run("仅剩文本形式的约束名", func(t *testing.T) {
		err := translateProductCodeConflict(
			errors.New(`ERROR: duplicate key value violates unique constraint "idx_catalog_products_code" (SQLSTATE 23505)`),
			"dup-code")
		if !errors.Is(err, ErrProductCodeExists) {
			t.Fatalf("文本形式也应被识别，得到 %v", err)
		}
	})

	// 关键防误报：SKU 编码的唯一约束不能被当成商品编码冲突，
	// 否则运营会去改商品编码，改完仍然报错。
	t.Run("其它唯一约束原样返回", func(t *testing.T) {
		orig := &pgconn.PgError{
			Code:           "23505",
			ConstraintName: "idx_catalog_product_specs_spec_code",
			Message:        "duplicate key value violates unique constraint \"idx_catalog_product_specs_spec_code\"",
		}
		got := translateProductCodeConflict(orig, "svc-1c1g")
		if !errors.Is(got, orig) {
			t.Fatalf("非商品编码冲突应原样返回，得到 %v", got)
		}
	})

	t.Run("非唯一约束错误原样返回", func(t *testing.T) {
		orig := errors.New("connection refused")
		if got := translateProductCodeConflict(orig, "x"); !errors.Is(got, orig) {
			t.Fatalf("无关错误应原样返回，得到 %v", got)
		}
	})
}
