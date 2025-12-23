package cmd

import "fmt"

const (
	DefaultClientTomlName = "client.toml"
	DefaultServerTomlName = "server.toml"
)

var AppVersion = "n/a"
var CommitHash = "n/a"
var BuildTimestamp = "n/a"

func BuildVersionOutput(appName string) string {
	return fmt.Sprintf("%s %s\nbuild: %s (%s)\n\n", appName, AppVersion, CommitHash, BuildTimestamp)
}
