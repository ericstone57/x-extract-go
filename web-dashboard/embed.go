package dashboard

import (
	"embed"
	"io/fs"
)

//go:embed all:build
var dashboardFS embed.FS

// GetDashboardFS returns the embedded Next.js dashboard filesystem
func GetDashboardFS() fs.FS {
	dashboard, _ := fs.Sub(dashboardFS, "build")
	return dashboard
}

