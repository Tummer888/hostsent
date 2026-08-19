package dto

type MenuNode struct {
	ID        uint64     `json:"id"`
	ParentID  uint64     `json:"parent_id"`
	Platform  string     `json:"platform"`
	Name      string     `json:"name"`
	Type      string     `json:"type"`
	Path      string     `json:"path,omitempty"`
	Component string     `json:"component,omitempty"`
	Icon      string     `json:"icon,omitempty"`
	SortOrder int        `json:"sort_order"`
	Status    string     `json:"status"`
	Children  []MenuNode `json:"children,omitempty"`
}
