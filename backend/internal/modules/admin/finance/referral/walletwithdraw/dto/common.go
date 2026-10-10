// Package dto 提供提现子域的通用数据传输结构。
package dto

// ListMeta 通用分页元信息
type ListMeta struct {
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
	Total    int64 `json:"total"`
}
