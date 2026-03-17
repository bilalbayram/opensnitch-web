package version

// Set at build time via -ldflags "-X .../version.Version=... -X .../version.BuildTime=..."
var (
	Version   = "dev"
	BuildTime = ""
)
