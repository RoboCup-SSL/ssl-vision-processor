module github.com/RoboCup-SSL/ssl-vision-processor/gui

go 1.27.3

require (
	github.com/lmittmann/tint v1.2.1
	google.golang.org/protobuf v1.36.12
	gopkg.in/natefinch/lumberjack.v2 v2.2.1
)

require gopkg.in/yaml.v3 v3.0.1

require (
	github.com/bluenviron/mediacommon/v2 v2.10.0
	github.com/gorilla/websocket v1.5.3
	github.com/pion/rtp v1.10.6
)

require (
	github.com/BurntSushi/toml v1.4.1-0.20240526193622-a339e1f7089c // indirect
	github.com/abema/go-mp4 v1.7.4 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/pion/randutil v0.1.0 // indirect
	github.com/swaggest/refl v1.4.0 // indirect
	golang.org/x/exp/typeparams v0.0.0-20231108232855-2478ac86f678 // indirect
	golang.org/x/mod v0.43.0 // indirect
	golang.org/x/sync v0.25.0 // indirect
	golang.org/x/telemetry v0.0.0-20261116191925-e77a43154acb // indirect
	golang.org/x/tools v0.53.0 // indirect
	golang.org/x/vuln v1.10.0 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	honnef.co/go/tools v0.8.2 // indirect
)

require (
	github.com/libp2p/go-reuseport v0.4.0
	github.com/swaggest/jsonschema-go v0.3.79
	github.com/swaggest/openapi-go v0.2.61
	golang.org/x/net v0.63.0
	golang.org/x/sys v0.50.0
)

tool (
	golang.org/x/vuln/cmd/govulncheck
	google.golang.org/protobuf/cmd/protoc-gen-go
	honnef.co/go/tools/cmd/staticcheck
)
