package service

import "errors"

var (
	ErrProductInvalidStatus = errors.New("非法的产品状态流转")
	// ErrSpecNotFound 规格（SKU）不存在或不属于该商品
	ErrSpecNotFound = errors.New("规格不存在")
	// ErrSpecCodeExists 同一商品（SPU）下规格（SKU）编码重复
	ErrSpecCodeExists = errors.New("规格编码已存在")
	// ErrProductCodeExists 商品编码已被占用（products.code 唯一索引，商品/SPU 级）。
	// 单独定义而不是复用 ErrSpecCodeExists：一个说的是商品编码（SPU）、一个是规格编码（SKU），
	// 提示语不同，混用会让运营分不清该改哪一个。
	ErrProductCodeExists = errors.New("商品编码已被占用")
	// ErrSpecInvalid 规格取值不符合原子字典约束
	ErrSpecInvalid = errors.New("规格不符合原子字典约束")
)
