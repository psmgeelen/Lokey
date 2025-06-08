module github.com/lokey/rng-service/cmd/api
module github.com/lokey/rng-service/cmd/api
module github.com/lokey/rng-service/cmd/api

go 1.24

replace github.com/lokey/rng-service => ../../

// Make sure pkg directory is accessible
replace github.com/lokey/rng-service/pkg => ../../pkg

require (
	github.com/gin-gonic/gin v1.10.1
	github.com/lokey/rng-service v0.0.0-00010101000000-000000000000
	github.com/lokey/rng-service/pkg v0.0.0-00010101000000-000000000000
)
go 1.24

replace github.com/lokey/rng-service => ../../

// Make sure pkg directory is accessible
replace github.com/lokey/rng-service/pkg => ../../pkg
go 1.24

replace github.com/lokey/rng-service => ../../

require github.com/lokey/rng-service v0.0.0-00010101000000-000000000000

require (
	github.com/KyleBanks/depth v1.2.1 // indirect
	github.com/apache/arrow/go/v14 v14.0.2 // indirect
	github.com/bytedance/sonic v1.11.6 // indirect
	github.com/chenzhuoyu/base64x v0.0.0-20230717121745-296ad89f973d // indirect
	github.com/chenzhuoyu/iasm v0.9.1 // indirect
	github.com/gabriel-vasile/mimetype v1.4.8 // indirect
	github.com/gin-contrib/sse v0.1.0 // indirect
	github.com/gin-gonic/gin v1.10.1 // indirect
	github.com/go-openapi/jsonpointer v0.20.2 // indirect
	github.com/go-openapi/jsonreference v0.20.4 // indirect
	github.com/go-openapi/spec v0.20.13 // indirect
	github.com/go-openapi/swag v0.22.7 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/go-playground/validator/v10 v10.26.0 // indirect
	github.com/goccy/go-json v0.10.2 // indirect
	github.com/google/flatbuffers v23.5.26+incompatible // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/compress v1.16.7 // indirect
	github.com/klauspost/cpuid/v2 v2.2.7 // indirect
	github.com/leodido/go-urn v1.4.0 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/marcboeker/go-duckdb v1.6.1 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/pelletier/go-toml/v2 v2.2.2 // indirect
	github.com/pierrec/lz4/v4 v4.1.18 // indirect
	github.com/swaggo/files v1.0.1 // indirect
	github.com/swaggo/gin-swagger v1.6.0 // indirect
	github.com/swaggo/swag v1.16.2 // indirect
	github.com/twitchyliquid64/golang-asm v0.15.1 // indirect
	github.com/ugorji/go/codec v1.2.12 // indirect
	github.com/zeebo/xxh3 v1.0.2 // indirect
	golang.org/x/arch v0.8.0 // indirect
	golang.org/x/crypto v0.33.0 // indirect
	golang.org/x/mod v0.17.0 // indirect
	golang.org/x/net v0.34.0 // indirect
	golang.org/x/sys v0.30.0 // indirect
	golang.org/x/text v0.22.0 // indirect
	golang.org/x/tools v0.21.1-0.20240508182429-e35e4ccd0d2d // indirect
	golang.org/x/xerrors v0.0.0-20220907171357-04be3eba64a2 // indirect
	google.golang.org/protobuf v1.34.1 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
