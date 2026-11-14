package email

import "embed"

// FS holds the panel's emails: the letter every email is drawn as (layout.html) and the texts of
// each email in every notification language (messages.<lang>.json).
//
//go:embed layout.html messages.*.json
var FS embed.FS
