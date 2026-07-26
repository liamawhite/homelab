// Package webui embeds the built React frontend (web/, built by `npm run
// build` into web/dist/, copied here by the Dockerfile/Makefile as
// internal/webui/dist/) into the Go binary so one process can serve both the
// Connect API and the static UI on one port.
package webui

import "embed"

//go:embed all:dist
var DistFS embed.FS
