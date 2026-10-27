// Package migrations embeds the proposed Notification SQL for explicit, reviewed application.
// Nothing here runs at service startup.
package migrations

import _ "embed"

//go:embed 001_init.sql
var Init string
