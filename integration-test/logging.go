package tests

import (
	"context"
	"io"
	"os"

	"github.com/rs/zerolog/log"
	"github.com/testcontainers/testcontainers-go"
)

// Streams the logs of a container to stdout.
func setupContainerLogging(ctx context.Context, container testcontainers.Container) {
	go func() {
		logReader, err := container.Logs(ctx)
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to get container logs")
			return
		}
		defer func() { _ = logReader.Close() }()

		containerName, err := container.Name(ctx)
		if err != nil {
			containerName = "container"
		}

		log.Info().Msgf("Started streaming logs for container: %s", containerName)

		_, err = io.Copy(os.Stdout, logReader)
		if err != nil && err != io.EOF {
			log.Fatal().Err(err).Msgf("Error streaming logs from container %s", containerName)
		}
	}()
}
