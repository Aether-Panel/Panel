package models

type ProvisionProduct struct {
	ID          uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	ProductID   string `gorm:"uniqueIndex;size:100" json:"product_id"` // "minecraft_2gb", "rust_4gb"
	DisplayName string `gorm:"size:150" json:"display_name"`
	Template    string `gorm:"size:100" json:"template"` // nombre del template en templates.aetherpanel.es
	NodeID      *uint  `json:"node_id"`                  // nil = auto-select, número = nodo específico
	CPU         int    `json:"cpu"`                      // e.g. 100 (= 1 core)
	Memory      int64  `json:"memory"`                   // MB
	Disk        int64  `json:"disk"`                     // MB
	DefaultNode uint   `json:"default_node"`             // qué nodo usar si NodeID es nil
}
