module github.com/auren/auren-transfer-agent

go 1.22

require (
	github.com/go-chi/chi/v5 v5.2.2
	github.com/rs/zerolog v1.33.0
	github.com/spf13/viper v1.20.1
)

replace github.com/spf13/viper => ./internal/config/vipercompat

replace github.com/rs/zerolog => ./internal/logger/zerologcompat

replace github.com/go-chi/chi/v5 => ./internal/server/chicompat
