module github.com/northerntechhq/nt-connect

go 1.24.0

replace github.com/urfave/cli/v2 => github.com/mendersoftware/cli/v2 v2.1.1-minimal

require (
	github.com/coder/websocket v1.8.14
	github.com/creack/pty v1.1.24
	github.com/go-ozzo/ozzo-validation/v4 v4.3.0
	github.com/mendersoftware/go-lib-micro v0.0.0-20250620123909-c9fc306420c6
	github.com/pkg/errors v0.9.1
	github.com/satori/go.uuid v1.2.0
	github.com/sirupsen/logrus v1.9.4
	github.com/stretchr/testify v1.11.1
	github.com/urfave/cli/v2 v2.27.7
	github.com/vmihailenco/msgpack/v5 v5.4.1
	golang.org/x/sys v0.39.0
)

require (
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/stretchr/objx v0.5.2 // indirect
	github.com/vmihailenco/tagparser/v2 v2.0.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
