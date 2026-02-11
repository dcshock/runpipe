module github.com/dcshock/runpipe/config

go 1.24.0

require (
	github.com/dcshock/runpipe v0.0.0
	gopkg.in/yaml.v3 v3.0.1
)

require github.com/google/uuid v1.6.0 // indirect

replace github.com/dcshock/runpipe => ..
