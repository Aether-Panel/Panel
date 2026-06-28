package models

type PanelSetting struct {
	Key   string `gorm:"primaryKey;size:100" json:"key"`
	Value string `gorm:"size:255" json:"value"`
}

func (PanelSetting) TableName() string {
	return "panel_settings"
}
