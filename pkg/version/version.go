package version

import "fmt"

var (
	version   = "snapshot"
	gitCommit = "unspecified"
)

type VersionInfo struct {
	Name      string
	Version   string
	GitCommit string
}

var Info = VersionInfo{
	Name: "ch-migrator-example",
}

func init() {
	Info.Version = version
	Info.GitCommit = gitCommit
}

func (v VersionInfo) String() string {
	return fmt.Sprintf("%s version=%s commit=%s", v.Name, v.Version, v.GitCommit)
}
