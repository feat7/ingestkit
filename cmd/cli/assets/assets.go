package assets

import _ "embed"

//go:embed docker-compose.server.yaml
var DockerComposeTemplate string

//go:embed env.server.template
var EnvTemplate string

//go:embed schema.server.template.yaml
var SchemaTemplate string
