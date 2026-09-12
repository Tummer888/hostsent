package dto

// PriceMatrixQuery 周期价格矩阵查询。
type PriceMatrixQuery struct {
	ProductID uint64 `form:"product_id" json:"product_id"`
	SpecID    uint64 `form:"spec_id" json:"spec_id"`
	Currency  string `form:"currency" json:"currency"`
}

// PriceMatrixItem 矩阵中的一档周期价格。
type PriceMatrixItem struct {
	Cycle        string  `json:"cycle"`
	CycleName    string  `json:"cycle_name"`
	Price        float64 `json:"price"`
	CostPrice    float64 `json:"cost_price"`
	SetupFee     float64 `json:"setup_fee"`
	CostSetupFee float64 `json:"cost_setup_fee"`
	Source       string  `json:"source"`
	// Status 1 启用 / 0 停用；上游不支持的周期恒为 0 且不可改。
	Status int `json:"status"`
	// Editable 是否允许人工改价：上游派生行为 false，自营/人工行为 true。
	Editable bool   `json:"editable"`
	Remark   string `json:"remark"`
}

// PriceMatrixResponse 商品（或某 SKU）的完整周期价格矩阵。
type PriceMatrixResponse struct {
	ProductID    uint64            `json:"product_id"`
	SpecID       uint64            `json:"spec_id"`
	ProductName  string            `json:"product_name"`
	Currency     string            `json:"currency"`
	SourceMode   string            `json:"source_mode"`
	UpstreamCycles []string        `json:"upstream_cycles"` // 上游可提供的周期（来自渠道能力声明）
	Items        []PriceMatrixItem `json:"items"`
}

// PriceMatrixSaveRequest 整表保存：提交的行 upsert，未提交的档位删除。
type PriceMatrixSaveRequest struct {
	ProductID uint64                 `json:"product_id" binding:"required"`
	SpecID    uint64                 `json:"spec_id"`
	Currency  string                 `json:"currency"`
	Remark    string                 `json:"remark"`
	Items     []PriceMatrixItemInput `json:"items"`
}

// PriceMatrixItemInput 保存入参中的单档。
type PriceMatrixItemInput struct {
	Cycle        string  `json:"cycle" binding:"required"`
	Price        float64 `json:"price"`
	CostPrice    float64 `json:"cost_price"`
	SetupFee     float64 `json:"setup_fee"`
	CostSetupFee float64 `json:"cost_setup_fee"`
	Status       int     `json:"status"`
}

// CyclePriceRow 由上游同步/克隆导入推导出的单档价格（内部调用，非 HTTP 入参）。
type CyclePriceRow struct {
	Cycle        string  `json:"cycle"`
	Price        float64 `json:"price"`
	CostPrice    float64 `json:"cost_price"`
	SetupFee     float64 `json:"setup_fee"`
	CostSetupFee float64 `json:"cost_setup_fee"`
	Source       string  `json:"source"` // upstream | markup；空则默认 upstream
}
