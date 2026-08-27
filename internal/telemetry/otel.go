package telemetry

import (
	"errors"

	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

const authorizationHeader = "Authorization"

type Config struct {
	Enabled     bool
	Endpoint    string
	Token       string
	ServiceName string
	Env         string
}

// getResource creates the resource that describes our application.
//
// You can add any attributes to your resource and all your metrics
// will contain those attributes automatically.
//
// There are some attributes that are very important to be added to the resource:
// 1. hostname: allows you to identify host-specific problems
// 2. version: allows you to pinpoint problems in specific versions.
func getResource(cfg Config) (*resource.Resource, error) {
	if cfg.ServiceName == "" {
		return nil, errors.New("service name is not defined")
	}
	resource := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName(cfg.ServiceName),
		semconv.DeploymentEnvironment(cfg.Env),
	)

	return resource, nil
}
