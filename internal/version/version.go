package version

// Set at build time via -ldflags "-X .../version.Version=... -X .../version.BuildTime=..."
var (
	Version   = "dev"
	BuildTime = ""
)

// ExitRestart signals "I updated, restart me" to the service manager.
// Pair with RestartForceExitStatus=42 in the systemd unit.
const ExitRestart = 42
