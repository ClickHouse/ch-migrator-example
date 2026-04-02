package migrations

import (
	"embed"
	"encoding/json"

	"github.com/rs/zerolog/log"
)

// RevisionLatest is the sentinel value meaning "migrate to the latest revision."
const RevisionLatest int64 = 0

type Config struct {
	DBAddr string
	DBPort string

	EnableTLS             bool
	ForceMergeTree        bool
	InsecureSkipTLSVerify bool
	UseHTTP               bool

	AllowMissing bool
	Revision     int64

	DBPassword string `json:"-"`
	DBUsername string
}

//go:embed migrations
var EmbeddedMigrations embed.FS

func (c Config) String() string {
	data, err := json.Marshal(c)
	if err != nil {
		log.Warn().Err(err).Msg("unable to print config")
	}
	return string(data)
}
