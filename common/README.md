# Common Actions

## [Healthcheck](./healthcheck)

Makes a `GET` or `HEAD` http request via `cURL` to a given endpoint. Returns the
status code as an output.

### Inputs

| Input                  | Description                                                                | Example                            |
| ---------------------- | -------------------------------------------------------------------------- | ---------------------------------- |
| `ENDPOINT`             | The url we should send the request to. Should be accessible to the runner. | `http://internal.service.nuon.run` |
| `METHOD`               | One of `GET` or `HEAD`                                                     | `GET` or `HEAD`                    |
| `EXPECTED_STATUS_CODE` | The HTTP status code that qualifies as a success.                          | `200`, `204`, `301`                |

### Outputs

Format: `json`

| Value         | Description                                                         | Example         |
| ------------- | ------------------------------------------------------------------- | --------------- |
| `status_code` | HTTP Response status code                                           | `200`           |
| `success`     | Wether or not the response status code matches expected status code | `true`, `false` |
| `indicator`   | A visual indicator of `success` for use in templates                | 🔴, 🟢          |

### Example Configuration

```toml
#:schema http://localhost:8081/v1/general/config-schema?source=action
name    = "healthcheck"
timeout = "5m0s"


[[triggers]]
type          = "cron"
cron_schedule = "*/5 * * * *"
# every five minutes - https://crontab.guru/#*/5_*_*_*_*

[[triggers]]
type = "manual"

[[steps]]
name    = "healthcheck"
command = "./healthcheck"

[steps.public_repo]
repo      = "nuonco/actions"
branch    = "main"
directory = "common/"

[steps.env_vars]
ENDPOINT             = "https://svc.{{.nuon.install.id}}.nuon.run"
METHOD               = "HEAD"
EXPECTED_STATUS_CODE = "200"
```
