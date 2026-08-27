// Package mofangyun 定义魔方云（MoFangYun）上游适配器的 API 类型。
//
// 真实魔方云 API 为面板式入口 `index.php?m=api&a=<方法>`，请求携带公共参数，
// 响应统一为 `{code, msg, data}`。具体鉴权字段名与各方法名以魔方云后台
// 「系统设置-开发文档」为准，本文件仅定义后续转换所需的响应结构。
package mofangyun

import "time"

// MoFangYunResp 魔方云统一响应外壳。
type MoFangYunResp[T any] struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data T      `json:"data"`
}

// MoFangYunProduct 魔方云商品（对应财务侧可配置项的套餐维度）。
type MoFangYunProduct struct {
	ID             string  `json:"id"`
	Name           string  `json:"name"`
	CPU            int     `json:"cpu_num"`
	Memory         int     `json:"memory_size"`
	Disk           int     `json:"disk_size"`
	DiskType       string  `json:"disk_type"`
	Bandwidth      int     `json:"bandwidth"`
	OSType         string  `json:"os_type"`
	Region         string  `json:"region"`
	Zone           string  `json:"zone"`
	MaxInstances   int     `json:"max_instances"`
	CostPrice      float64 `json:"cost_price"`
	SuggestedPrice float64 `json:"suggested_price"`
	Status         string  `json:"status"`
}

// MoFangYunInstance 魔方云实例（云主机）。
type MoFangYunInstance struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CPU       int       `json:"cpu_num"`
	Memory    int       `json:"memory_size"`
	Disk      int       `json:"disk_size"`
	DiskType  string    `json:"disk_type"`
	Bandwidth int       `json:"bandwidth"`
	State     string    `json:"state"`
	PrivateIP string    `json:"private_ip"`
	PublicIP  string    `json:"public_ip"`
	Region    string    `json:"region"`
	Zone      string    `json:"zone"`
	Host      string    `json:"host"`
	OsImage   string    `json:"os_image"`
	CreatedAt time.Time `json:"created_at"`
	ExpireAt  time.Time `json:"expire_at"`
}

// MoFangYunNode 魔方云节点，作为统一资源池的映射来源（魔方云无独立「资源池」接口）。
type MoFangYunNode struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Region    string `json:"region"`
	Type      string `json:"type"` // host/lightHost/hyperv/adsl
	TotalCPU  int    `json:"total_cpu"`
	TotalMem  int    `json:"total_memory"`
	TotalDisk int    `json:"total_disk"`
	UsedCPU   int    `json:"used_cpu"`
	UsedMem   int    `json:"used_memory"`
	UsedDisk  int    `json:"used_disk"`
	Status    string `json:"status"`
}

// MoFangYunAccount 魔方云账户资源信息。
type MoFangYunAccount struct {
	TotalCPU    int     `json:"total_cpu"`
	TotalMemory int     `json:"total_memory"`
	TotalDisk   int     `json:"total_disk"`
	UsedCPU     int     `json:"used_cpu"`
	UsedMemory  int     `json:"used_memory"`
	UsedDisk    int     `json:"used_disk"`
	Balance     float64 `json:"balance"`
	Currency    string  `json:"currency"`
}
