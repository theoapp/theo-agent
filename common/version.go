package common

import (
	"fmt"
	"runtime"
)

var NAME = "theo-agent"
var VERSION = "development version"
var REVISION = "HEAD"
var BRANCH = "HEAD"
var BUILT = "unknown"

var AppVersion AppVersionInfo

type AppVersionInfo struct {
	Name         string
	Version      string
	Revision     string
	Branch       string
	GOVersion    string
	BuiltAt      string
	OS           string
	Architecture string
}

func (v *AppVersionInfo) Printer() {
	fmt.Print(v.extended())
}

func (v *AppVersionInfo) UserAgent() string {
	return fmt.Sprintf("%s %s (%s; %s; %s/%s)", v.Name, v.Version, v.Branch, v.GOVersion, v.OS, v.Architecture)
}

func (v *AppVersionInfo) extended() string {
	version := fmt.Sprintf("Version:      %s\n", v.Version)
	version += fmt.Sprintf("Git revision: %s\n", v.Revision)
	version += fmt.Sprintf("Git branch:   %s\n", v.Branch)
	version += fmt.Sprintf("GO version:   %s\n", v.GOVersion)
	version += fmt.Sprintf("Built:        %s\n", v.BuiltAt)
	version += fmt.Sprintf("OS/Arch:      %s/%s\n", v.OS, v.Architecture)

	return version
}

func init() {
	AppVersion = AppVersionInfo{
		Name:         NAME,
		Version:      VERSION,
		Revision:     REVISION,
		Branch:       BRANCH,
		GOVersion:    runtime.Version(),
		BuiltAt:      BUILT,
		OS:           runtime.GOOS,
		Architecture: runtime.GOARCH,
	}
}
