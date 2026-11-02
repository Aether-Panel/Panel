package models

import "time"

// NodeTransfer is a move of a server, with its sub-servers, to another node of this panel
// that has not ended yet. It is kept while the transfer runs, so a panel that restarts in
// the middle can undo it, or finish it when the panel already points at the new node.
type NodeTransfer struct {
	ID uint `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	// ServerID is the server the transfer was asked for; its sub-servers move with it
	ServerID     string `gorm:"column:server_id;size:20;not null;uniqueIndex" json:"-"`
	TargetNodeID uint   `gorm:"column:target_node_id;not null" json:"-"`
	// UserID is who asked for it, so what happens to it is recorded under their name
	UserID *uint `gorm:"column:user_id" json:"-"`
	// Committed is set in the same transaction that points the servers at the new node
	Committed bool `gorm:"column:committed;not null;default:false" json:"-"`
	// Moves is each server being moved and how far it got
	Moves []NodeTransferMove `gorm:"column:moves;type:text;serializer:json" json:"-"`

	CreatedAt time.Time `json:"-"`
	UpdatedAt time.Time `json:"-"`
}

// NodeTransferMove is one server of a NodeTransfer
type NodeTransferMove struct {
	ServerID     string `json:"serverId"`
	SourceNodeID uint   `json:"sourceNodeId"`
	// Step is the last step the transfer began for the server
	Step string `json:"step"`
	// WasRunning is whether the server ran before the transfer stopped it
	WasRunning bool `json:"wasRunning"`
	// CopiedBackups are the backups written, or being written, on the new node
	CopiedBackups []string `json:"copiedBackups,omitempty"`
}
