package docker

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/deepsourcelabs/cli/analyzers/config"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/archive"
)

type ErrorLine struct {
	Error       string      `json:"error"`
	ErrorDetail ErrorDetail `json:"errorDetail"`
}

type ErrorDetail struct {
	Message string `json:"message"`
}

type DockerBuildResponse struct {
	Stream string `json:"stream"`
}

type AnalysisParams struct {
	AnalysisCommand   string
	HostCodePath      string
	ContainerCodePath string
	ToolboxPath       string
}

type DockerClient struct {
	Client         *client.Client
	ContainerName  string
	ImageName      string
	ImageTag       string
	DockerfilePath string
	AnalysisOpts   AnalysisParams
}

type DockerBuildError struct {
	Message string
}

func (d *DockerBuildError) Error() string {
	return d.Message
}

func (d *DockerClient) BuildAnalyzerDockerImage() *DockerBuildError {
	var err error
	d.Client, err = client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return &DockerBuildError{
			Message: err.Error(),
		}
	}

	if err = d.executeImageBuild(); err != nil {
		return &DockerBuildError{
			Message: err.Error(),
		}
	}
	return nil
}

func (d *DockerClient) executeImageBuild() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*120)
	defer cancel()
	cwd, _ := os.Getwd()

	tarOptions := &archive.TarOptions{
		ExcludePatterns: []string{".git/**"},
	}
	tar, err := archive.TarWithOptions(cwd, tarOptions)
	if err != nil {
		return err
	}

	opts := types.ImageBuildOptions{
		Dockerfile: d.DockerfilePath,
		Tags:       []string{fmt.Sprintf("%s:%s", d.ImageName, d.ImageTag)},
		Remove:     true,
		Platform:   "linux",
	}
	res, err := d.Client.ImageBuild(ctx, tar, opts)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	return checkResponse(res.Body)
}

func checkResponse(rd io.Reader) error {
	var lastLine []byte
	count := 0
	var currentStream string

	scanner := bufio.NewScanner(rd)
	for scanner.Scan() {
		lastLine = scanner.Bytes()
		d := &DockerBuildResponse{}
		err := json.Unmarshal(lastLine, d)
		if err != nil {
			return err
		}
		if d.Stream == "" || d.Stream == "\n" || strings.TrimSuffix(d.Stream, "\n") == currentStream {
			continue
		}
		currentStream = strings.TrimSuffix(d.Stream, "\n")
		count++
	}

	errLine := &ErrorLine{}
	json.Unmarshal([]byte(lastLine), errLine)
	if errLine.Error != "" {
		return errors.New(errLine.Error)
	}
	return scanner.Err()
}

// Returns the docker image details to build
func GetDockerImageDetails(analyzerTOMLData *config.AnalyzerMetadata) (string, string) {
	var dockerFilePath, dockerFileName string
	dockerFilePath = "Dockerfile"

	// Read config for the value if specified
	if analyzerTOMLData.Build.Dockerfile != "" {
		dockerFilePath = analyzerTOMLData.Build.Dockerfile
	}
	if analyzerTOMLData.Shortcode != "" {
		dockerFileName = strings.TrimPrefix(analyzerTOMLData.Shortcode, "@")
	}
	return dockerFilePath, dockerFileName
}

func GenerateImageVersion(length int) string {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return fmt.Sprintf("%x", b)
}
