module github.com/mavioai/mavio/apps/server

go 1.27.0

require (
	github.com/mavioai/mavio/libs/core v0.0.0
	github.com/mavioai/mavio/libs/imaging v0.0.0
	github.com/mavioai/mavio/libs/media v0.0.0
	github.com/mavioai/mavio/libs/metadata v0.0.0
	github.com/mavioai/mavio/libs/naming v0.0.0
	github.com/mavioai/mavio/libs/plugin v0.0.0
	github.com/mavioai/mavio/libs/proto v0.0.0
	github.com/mavioai/mavio/libs/store v0.0.0
	github.com/mavioai/mavio/libs/subtitle v0.0.0
	github.com/mavioai/mavio/tools/fixtures v0.0.0
	google.golang.org/protobuf v1.36.12
)

require (
	ariga.io/atlas v0.36.2-0.20250730182955-2c6300d0a3e1 // indirect
	connectrpc.com/connect v1.21.0 // indirect
	entgo.io/ent v0.14.6 // indirect
	github.com/agext/levenshtein v1.2.3 // indirect
	github.com/apparentlymart/go-textseg/v15 v15.0.0 // indirect
	github.com/bmatcuk/doublestar v1.3.4 // indirect
	github.com/dlclark/regexp2 v1.12.0 // indirect
	github.com/go-openapi/inflect v0.19.0 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/hashicorp/hcl/v2 v2.18.1 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/pgx/v5 v5.11.0 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/lib/pq v1.10.9 // indirect
	github.com/mitchellh/go-wordwrap v1.0.1 // indirect
	github.com/mozillazg/go-unidecode v0.2.0 // indirect
	github.com/ncruces/go-sqlite3 v0.35.6 // indirect
	github.com/ncruces/go-sqlite3-wasm/v6 v6.3.35304 // indirect
	github.com/ncruces/julianday v1.0.0 // indirect
	github.com/santhosh-tekuri/jsonschema/v6 v6.0.3 // indirect
	github.com/tetratelabs/wazero v1.12.0 // indirect
	github.com/wlynxg/chardet v1.0.5 // indirect
	github.com/zclconf/go-cty v1.14.4 // indirect
	github.com/zclconf/go-cty-yaml v1.1.0 // indirect
	golang.org/x/image v0.46.0 // indirect
	golang.org/x/mod v0.41.0 // indirect
	golang.org/x/sync v0.23.0 // indirect
	golang.org/x/sys v0.48.0 // indirect
	golang.org/x/text v0.42.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/mavioai/mavio/libs/proto => ../../libs/proto

replace github.com/mavioai/mavio/libs/core => ../../libs/core

replace github.com/mavioai/mavio/libs/store => ../../libs/store

replace github.com/mavioai/mavio/libs/plugin => ../../libs/plugin

replace github.com/mavioai/mavio/libs/naming => ../../libs/naming

replace github.com/mavioai/mavio/libs/metadata => ../../libs/metadata

replace github.com/mavioai/mavio/libs/subtitle => ../../libs/subtitle

replace github.com/mavioai/mavio/libs/imaging => ../../libs/imaging

replace github.com/mavioai/mavio/libs/media => ../../libs/media

replace github.com/mavioai/mavio/tools/fixtures => ../../tools/fixtures
