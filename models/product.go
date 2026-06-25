package models

type ProvisionProduct struct {
	ID           uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	ProductID    string `gorm:"uniqueIndex;size:100" json:"product_id"`   // "minecraft_2gb", "rust_4gb"
	DisplayName  string `gorm:"size:150" json:"display_name"`
	Template     string `gorm:"size:100" json:"template"`                 // nombre del template
	NodeID       *uint  `json:"node_id"`                                  // nil = auto-select
	CPU          int    `json:"cpu"`                                      // e.g. 100 (= 1 core)
	Memory       int64  `json:"memory"`                                   // MB
	Disk         int64  `json:"disk"`                                     // MB
	DefaultNode  uint   `json:"default_node"`                             // nodo fallback
	PortRangeMin uint16 `gorm:"default:0" json:"port_range_min"`          // 0 = sin rango asignado
	PortRangeMax uint16 `gorm:"default:0" json:"port_range_max"`          // 0 = sin rango asignado
}
