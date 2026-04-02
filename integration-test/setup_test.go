package tests

import (
	"bytes"
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	clickhouse "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/docker/go-connections/nat"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	clickhouseImage        = "clickhouse/clickhouse-server:25.8"
	clickhouseInternalPort = "9000/tcp"
)

//go:embed clickhouse-users.xml
var clickhouseUsersXMLContent []byte

//go:embed clickhouse-logger.xml
var clickhouseLoggerXMLContent []byte

//go:embed default-password.xml
var defaultPasswordXMLContent []byte

//go:embed clickhouse-systables.xml
var clickhouseSysTablesContent []byte

//go:embed init.sql
var initSQL []byte

// Common container info to allow separate database connections
var (
	clickhouseHost string
	clickhousePort string

	// Keep for backward compatibility
	sharedConn *sql.DB
)

type ConfigFile struct {
	Content       []byte
	NamePattern   string
	ContainerPath string
	FileMode      int64
}

func TestMain(m *testing.M) {
	ctx := context.Background()

	tempDockerConfigDir, err := os.MkdirTemp("", "testcontainers-docker-config-")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create temporary DOCKER_CONFIG directory")
	}
	defer func() {
		if err := os.RemoveAll(tempDockerConfigDir); err != nil {
			log.Error().Err(err).Str("path", tempDockerConfigDir).Msg("Failed to remove temporary DOCKER_CONFIG directory")
		} else {
			log.Info().Str("path", tempDockerConfigDir).Msg("Successfully removed temporary DOCKER_CONFIG directory")
		}
	}()

	minimalConfig := map[string]interface{}{}
	configData, err := json.Marshal(minimalConfig)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to marshal minimal Docker config")
	}

	configFilePath := filepath.Join(tempDockerConfigDir, "config.json")
	if err := os.WriteFile(configFilePath, configData, 0o600); err != nil {
		log.Fatal().Err(err).Msg("Failed to write minimal Docker config.json")
	}

	if err := os.Setenv("DOCKER_CONFIG", tempDockerConfigDir); err != nil {
		log.Fatal().Err(err).Msg("Failed to set temporary DOCKER_CONFIG environment variable")
	}
	log.Info().Str("DOCKER_CONFIG", tempDockerConfigDir).Msg("Set temporary DOCKER_CONFIG to isolate credential helper behavior")

	configFiles := []ConfigFile{
		{
			Content:       clickhouseUsersXMLContent,
			NamePattern:   "users-*.xml",
			ContainerPath: "/etc/clickhouse-server/config.d/users.xml",
			FileMode:      0o644,
		},
		{
			Content:       clickhouseLoggerXMLContent,
			NamePattern:   "logger-*.xml",
			ContainerPath: "/etc/clickhouse-server/config.d/logger.xml",
			FileMode:      0o644,
		},
		{
			Content:       clickhouseSysTablesContent,
			NamePattern:   "systables-*.xml",
			ContainerPath: "/etc/clickhouse-server/config.d/system-tables.xml",
			FileMode:      0o644,
		},
		{
			Content:       defaultPasswordXMLContent,
			NamePattern:   "password-*.xml",
			ContainerPath: "/etc/clickhouse-server/users.d/default-password.xml",
			FileMode:      0o644,
		},
		{
			Content:       initSQL,
			NamePattern:   "init.sql",
			ContainerPath: "/docker-entrypoint-initdb.d/init.sql",
			FileMode:      0o644,
		},
	}

	containerFiles := make([]testcontainers.ContainerFile, 0, len(configFiles))
	for _, cf := range configFiles {
		containerFiles = append(containerFiles, testcontainers.ContainerFile{
			Reader:            bytes.NewReader(cf.Content),
			ContainerFilePath: cf.ContainerPath,
			FileMode:          cf.FileMode,
		})
	}

	req := testcontainers.ContainerRequest{
		Image:        clickhouseImage,
		ExposedPorts: []string{clickhouseInternalPort},
		Name:         fmt.Sprintf("clickhouse-server-tc-%s", uuid.New().String()),
		Files:        containerFiles,
		Env: map[string]string{
			"CLICKHOUSE_PASSWORD":        "Demo",
			"CLICKHOUSE_SKIP_USER_SETUP": "1",
		},
		WaitingFor: wait.ForAll(
			wait.ForListeningPort(nat.Port(clickhouseInternalPort)),
			wait.ForSQL(clickhouseInternalPort, "clickhouse", func(host string, port nat.Port) string {
				return fmt.Sprintf("clickhouse://default:Demo@%s:%s/default?dial_timeout=5s", host, port.Port())
			}),
		).WithDeadline(2 * time.Minute),
	}

	log.Info().Msg("Starting ClickHouse container with Testcontainers...")
	clickHouseContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to start ClickHouse container")
	}
	setupContainerLogging(ctx, clickHouseContainer)

	defer func() {
		log.Info().Msg("Terminating ClickHouse container...")
		if err := clickHouseContainer.Terminate(ctx); err != nil {
			log.Error().Err(err).Msg("Failed to terminate ClickHouse container")
		} else {
			log.Info().Msg("ClickHouse container terminated successfully.")
		}
	}()

	host, err := clickHouseContainer.Host(ctx)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to get ClickHouse container host")
	}
	mappedPort, err := clickHouseContainer.MappedPort(ctx, nat.Port(clickhouseInternalPort))
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to get ClickHouse container mapped port")
	}

	log.Info().Str("host", host).Str("port", mappedPort.Port()).Msg("ClickHouse container is up and ready")

	// Store container info for test database creation
	clickhouseHost = host
	clickhousePort = mappedPort.Port()

	// Create default connection for backward compatibility
	sharedConn, err = buildClickHouseClient(host, mappedPort.Port(), "default")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to ClickHouse")
	}

	m.Run()
}

// GetTestDBConnection creates a connection to a dedicated database for a test suite
// Each test suite should use this with a unique dbName to ensure isolation
func GetTestDBConnection(t *testing.T, dbName string) *sql.DB {
	t.Helper()

	// Need to connect to default DB first to create the test DB
	defaultConn, err := buildClickHouseClient(clickhouseHost, clickhousePort, "default")
	if err != nil {
		t.Fatalf("Failed to connect to default ClickHouse database: %v", err)
	}

	// Create a unique database for this test suite
	_, err = defaultConn.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS %s", dbName))
	if err != nil {
		t.Fatalf("Failed to drop existing database %s: %v", dbName, err)
	}

	_, err = defaultConn.Exec(fmt.Sprintf("CREATE DATABASE %s", dbName))
	if err != nil {
		t.Fatalf("Failed to create database %s: %v", dbName, err)
	}

	// Close default connection
	_ = defaultConn.Close()

	// Create a connection to the test database
	conn, err := buildClickHouseClient(clickhouseHost, clickhousePort, dbName)
	if err != nil {
		t.Fatalf("Failed to connect to test database %s: %v", dbName, err)
	}

	// Register cleanup to drop the database when test is done
	t.Cleanup(func() {
		_ = conn.Close()

		// Drop the database after test is done
		cleanupConn, err := buildClickHouseClient(clickhouseHost, clickhousePort, "default")
		if err != nil {
			t.Logf("Warning: Failed to connect for cleanup: %v", err)
			return
		}
		defer func() { _ = cleanupConn.Close() }()

		_, err = cleanupConn.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS %s", dbName))
		if err != nil {
			t.Logf("Warning: Failed to drop database %s during cleanup: %v", dbName, err)
		}
	})

	return conn
}

func buildClickHouseClient(host, portValue, database string) (*sql.DB, error) {
	chOpts := clickhouse.Options{
		Addr: []string{fmt.Sprintf("%s:%s", host, portValue)},
		Auth: clickhouse.Auth{
			Database: database,
			Username: "default",
			Password: "Demo", // hardcoded from clickhouse-users.xml
		},
		DialTimeout: time.Second * 30,
		Compression: &clickhouse.Compression{
			Method: clickhouse.CompressionLZ4,
		},
		Settings: clickhouse.Settings{
			"max_execution_time": 60,
		},
	}
	log.Info().Msgf("Connecting to ClickHouse at %s:%s database %s", host, portValue, database)
	conn := clickhouse.OpenDB(&chOpts)
	if err := conn.Ping(); err != nil {
		var clickhouseErr *clickhouse.Exception
		if errors.As(err, &clickhouseErr) {
			log.Error().Msgf("ClickHouse exception during ping: Code: %d, Message: %s, StackTrace: %s", clickhouseErr.Code, clickhouseErr.Message, clickhouseErr.StackTrace)
		}
		return nil, fmt.Errorf("failed to ping ClickHouse: %w", err)
	}
	log.Info().Msgf("Successfully connected to ClickHouse database %s", database)
	return conn, nil
}
