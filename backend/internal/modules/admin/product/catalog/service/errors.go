package service

import "errors"

var (
	ErrProductInvalidStatus = errors.New("非法的产品状态流转")
	// ErrSpecNotFound SKU 不存在或不属于该商品
	ErrSpecNotFound = errors.New("规格不存在")
	// ErrSpecCodeExists 同一商品下规格编码重复
	ErrSpecCodeExists = errors.New("规格编码已存在")
	// ErrSpecInvalid 规格取值不符合原子字典约束
	ErrSpecInvalid = errors.New("规格不符合原子字典约束")
)
