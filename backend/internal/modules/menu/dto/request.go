package dto

type MenuCreateRequest struct {
	ParentID  uint64 `json:"parent_id"`
	Platform  string `json:"platform" binding:"required"`
	Name      string `json:"name" binding:"required"`
	Type      string `json:"type"`
	Path      string `json:"path"`
	Component string `json:"component"`
	Icon      string `json:"icon"`
	SortOrder int    `json:"sort_order"`
	Status    string `json:"status"`
}

type MenuUpdateRequest struct {
	ParentID  uint64 `json:"parent_id"`
	Platform  string `json:"platform" binding:"required"`
	Name      string `json:"name" binding:"required"`
	Type      string `json:"type"`
	Path      string `json:"path"`
	Component string `json:"component"`
	Icon      string `json:"icon"`
	SortOrder int    `json:"sort_order"`
	Status    string `json:"status" binding:"required"`
}
