module github.com/ravicb765/rca-app/tests/integration

go 1.21

require (
	github.com/stretchr/testify v1.8.4
	github.com/testcontainers/testcontainers-go v0.23.0
    github.com/testcontainers/testcontainers-go/modules/clickhouse v0.23.0
    github.com/testcontainers/testcontainers-go/modules/kafka v0.23.0
    github.com/testcontainers/testcontainers-go/modules/redis v0.23.0
    github.com/ravicb765/rca-app/server v0.0.0
)

replace github.com/ravicb765/rca-app/server => ../../server
