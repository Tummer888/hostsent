// Package dto 提供代理等级（agent level）子域的数据传输结构。
package dto

// 折扣目标类型（与 model.Target* 同值，导出给 handler 校验用）。
const (
	TargetCategory = "category"
	TargetProduct  = "product"
)

// ListQuery 代理等级列表查询。
type ListQuery struct {
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
	Status   string `form:"status"`
	Keyword  string `form:"keyword"`
}

// DiscountItemRequest 折扣矩阵的一格。
//
// TargetType=category/product 时 TargetID 必填；TargetType=all 时 TargetID 固定 0。
// DiscountRate 为折扣率（0.85 = 八五折，越小越优惠）；0 表示未配置。
type DiscountItemRequest struct {
	TargetType   string  `json:"target_type" binding:"omitempty,oneof=all category product"`
	TargetID     uint64  `json:"target_id"`
	DiscountRate float64 `json:"discount_rate"`
}

// CreateRequest 创建代理等级（可一并提交折扣矩阵）。
type CreateRequest struct {
	Name        string                `json:"name" binding:"required,min=1,max=64"`
	Code        string                `json:"code" binding:"required,min=1,max=64"`
	Weight      int                   `json:"weight"`
	Status      string                `json:"status"`
	Description string                `json:"description"`
	Discounts   []DiscountItemRequest `json:"discounts"`
}

// UpdateRequest 更新代理等级（折扣矩阵整体覆盖语义）。
type UpdateRequest struct {
	Name        string `json:"name" binding:"required,min=1,max=64"`
	Code        string `json:"code" binding:"required,min=1,max=64"`
	Weight      int    `json:"weight"`
	Status      string `json:"status" binding:"required"`
	Description string `json:"description"`
	// Discounts 为 nil 表示不修改折扣矩阵；非 nil（含空数组）表示整体覆盖。
	Discounts []DiscountItemRequest `json:"discounts"`
}

// DiscountItemInfo 折扣矩阵一格的展示结构（带目标名，便于前端免二次查询）。
type DiscountItemInfo struct {
	TargetType   string  `json:"target_type"`
	TargetID     uint64  `json:"target_id"`
	TargetName   string  `json:"target_name"`
	DiscountRate float64 `json:"discount_rate"`
}

// CellUpdateRequest 矩阵单格更新：在折扣矩阵上直接改某一格。
//
// 与 ApplyLadder（整行展开）互补：运营微调单个等级在某个目标上的折扣时用这个，
// 不必为了改一格把整行重填一遍。
type CellUpdateRequest struct {
	AgentLevelID uint64 `json:"agent_level_id" binding:"required"`
	TargetType   string `json:"target_type" binding:"required,oneof=all category product"`
	TargetID     uint64 `json:"target_id"`
	// DiscountRate 折扣率（0.85 = 八五折）；0 表示清除该格（不打折）。
	DiscountRate float64 `json:"discount_rate"`
}

// Info 代理等级信息。
type Info struct {
	ID            uint64             `json:"id"`
	Name          string             `json:"name"`
	Code          string             `json:"code"`
	Weight        int                `json:"weight"`
	Status        string             `json:"status"`
	Description   string             `json:"description"`
	MemberCount   int64              `json:"member_count"`
	DiscountCount int64              `json:"discount_count"`
	Discounts     []DiscountItemInfo `json:"discounts"`
	CreatedAt     string             `json:"created_at"`
	UpdatedAt     string             `json:"updated_at"`
}

// ListMeta 分页元信息。
type ListMeta struct {
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
	Total    int64 `json:"total"`
}

// ListResponse 代理等级列表响应。
type ListResponse struct {
	Items []Info   `json:"items"`
	Meta  ListMeta `json:"meta"`
}

// MatrixCell 折扣矩阵单元格（矩阵视图用：行列直接对齐，前端不必自己拼）。
type MatrixCell struct {
	AgentLevelID uint64  `json:"agent_level_id"`
	DiscountRate float64 `json:"discount_rate"`
	Configured   bool    `json:"configured"`
}

// MatrixRow 折扣矩阵的一行（一个分类 / 全站兜底）。
type MatrixRow struct {
	TargetType string `json:"target_type"`
	TargetID   uint64 `json:"target_id"`
	TargetName string `json:"target_name"`
	// CostRate 分类成本率（0 = 未配置）；全站行固定 0。
	CostRate float64      `json:"cost_rate"`
	Cells    []MatrixCell `json:"cells"`
}

// MatrixColumn 矩阵的列（一个代理等级）。
type MatrixColumn struct {
	AgentLevelID uint64 `json:"agent_level_id"`
	Name         string `json:"name"`
	Code         string `json:"code"`
	Weight       int    `json:"weight"`
	Status       string `json:"status"`
}

// MatrixResponse 折扣矩阵视图：列为代理等级（按权重降序），行为目标。
type MatrixResponse struct {
	Columns []MatrixColumn `json:"columns"`
	Rows    []MatrixRow    `json:"rows"`
	// ProductRows 商品例外行（doc108 §8E）：只列**配置过商品级折扣**的商品。
	// 商品级阶梯应用后运营必须能直接在矩阵上看到"所有等级都排好了"，
	// 否则看起来像只改了一个等级。
	ProductRows []MatrixRow `json:"product_rows"`
}

// LadderPreviewRequest 阶梯填充预览：按"锚点 + 步长"展开各等级的折扣率。
type LadderPreviewRequest struct {
	// AnchorRate 最优等级（权重最高）的折扣率，如 0.7 = 七折。
	AnchorRate float64 `json:"anchor_rate" binding:"required"`
	// Step 每往下一级增加的折扣率（让利变少），如 0.05。
	Step float64 `json:"step"`
	// TargetType 阶梯作用的目标：all / category / product。空 = all。
	// 商品级阶梯（doc108 §8E）：给单个商品配一套逐级折扣，不必逐等级手填。
	TargetType string `json:"target_type" binding:"omitempty,oneof=all category product"`
	TargetID   uint64 `json:"target_id"`
	// CostRate 可选的成本率覆盖；通常不传 —— 成本基准由服务端按目标解析
	// （分类读 cost_rate，商品按 cost_price/price），保证预览与落库同一条成本口径。
	CostRate float64 `json:"cost_rate"`
}

// LadderPreviewCell 阶梯预览的一格。
type LadderPreviewCell struct {
	AgentLevelID uint64  `json:"agent_level_id"`
	Name         string  `json:"name"`
	Weight       int     `json:"weight"`
	DiscountRate float64 `json:"discount_rate"`
	// GrossMargin 毛利率（折扣率 − 成本率）；CostRate=0 时为 0（无基准）。
	GrossMargin float64 `json:"gross_margin"`
	// Feasible 是否不低于成本（CostRate=0 时恒为 true）。
	Feasible bool `json:"feasible"`
}

// LadderPreviewResponse 阶梯填充预览结果。
type LadderPreviewResponse struct {
	Cells []LadderPreviewCell `json:"cells"`
	// Warnings 命中约束时的中文提示（前端直接展示）。
	Warnings []string `json:"warnings"`
}

// ApplyLadderRequest 应用阶梯：把展开结果写入某目标下的所有等级。
type ApplyLadderRequest struct {
	TargetType string  `json:"target_type" binding:"required,oneof=category product all"`
	TargetID   uint64  `json:"target_id"`
	AnchorRate float64 `json:"anchor_rate" binding:"required"`
	Step       float64 `json:"step"`
}
