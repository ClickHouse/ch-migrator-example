package main

import (
	"crypto/tls"
	"flag"
	"fmt"

	migrations "github.com/ClickHouse/ch-migrator-example/pkg"
	"github.com/ClickHouse/ch-migrator-example/pkg/version"

	// Register the bundled Go migrations (example_set_ttl, example_add_column).
	// Library consumers can omit this import and register their own migrations instead.
	_ "github.com/ClickHouse/ch-migrator-example/pkg/migrations"

	clickhouse "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/rs/zerolog/log"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

func main() {
	log.Info().Str("version", version.Info.String()).Msg("starting ch-migrator-example")

	mainCfg, err := loadConfig()
	if err != nil {
		log.Fatal().Err(err).Msg("error loading config ")
	}

	options := &clickhouse.Options{
		Addr: []string{fmt.Sprintf("%v:%v", mainCfg.DBAddr, mainCfg.DBPort)},
		Auth: clickhouse.Auth{
			Username: mainCfg.DBUsername,
			Password: mainCfg.DBPassword,
		},
		ConnMaxLifetime: 0,
		MaxOpenConns:    0,
	}
	if mainCfg.UseHTTP {
		options.Protocol = clickhouse.HTTP
	}
	if mainCfg.EnableTLS {
		options.TLS = &tls.Config{
			InsecureSkipVerify: mainCfg.InsecureSkipTLSVerify,
		}
	}
	db := clickhouse.OpenDB(options)
	log.Info().Any("config", mainCfg).Msg("Starting migration(s)")

	err = migrations.InitiateMigrations(db, mainCfg, migrations.EmbeddedMigrations)
	if err != nil {
		log.Fatal().Err(err).Msg("migration failed")
	}
}

func loadConfig() (migrations.Config, error) {
	flag.String("config", "", "Path to the config file")
	flag.Bool("allowMissing", false, "Allows goose to apply a migration out of order; should only be used when you know what migration is missing")
	flag.Int64("revision", migrations.RevisionLatest, "Migration revision")
	flag.String("dbAddr", "", "Database address (required)")
	flag.String("dbPort", "", "Database port (required)")
	flag.String("dbUsername", "default", "Database username to run migrations with")
	flag.String("dbPassword", "", "Database password to run migrations with")
	flag.Bool("enableTLS", true, "enable TLS")
	flag.Bool("insecureSkipTLSVerify", true, "skip TLS verify")
	flag.Bool("useHTTP", false, "Use HTTP protocol instead of native (for HTTPS on port 8443)")

	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	pflag.Parse()

	err := viper.BindPFlags(pflag.CommandLine)
	if err != nil {
		return migrations.Config{}, err
	}
	viper.AutomaticEnv()

	if configFile := viper.GetString("config"); configFile != "" {
		viper.SetConfigFile(configFile)
		if err = viper.ReadInConfig(); err != nil {
			return migrations.Config{}, fmt.Errorf("read config file: %w", err)
		}
	}

	return migrations.Config{
		DBAddr: viper.GetString("dbAddr"),
		DBPort: viper.GetString("dbPort"),

		EnableTLS:             viper.GetBool("enableTLS"),
		InsecureSkipTLSVerify: viper.GetBool("insecureSkipTLSVerify"),
		UseHTTP:               viper.GetBool("useHTTP"),

		DBUsername: viper.GetString("dbUsername"),
		DBPassword: viper.GetString("dbPassword"),

		AllowMissing: viper.GetBool("allowMissing"),
		Revision:     viper.GetInt64("revision"),
	}, nil
}
