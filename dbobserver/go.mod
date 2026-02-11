module github.com/dcshock/runpipe/dbobserver

go 1.24.0

require (
	github.com/dcshock/runpipe v0.0.0
	github.com/jackc/pgx/v5 v5.8.0
)

require (
	github.com/google/uuid v1.6.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	golang.org/x/text v0.29.0 // indirect
)

replace github.com/dcshock/runpipe => ..
