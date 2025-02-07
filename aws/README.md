# AWS Actions

## [ALB Healthcheck](./alb-healthcheck)

Check the status of the
[AWS ALB Ingresss nuon/component](https://github.com/nuonco/components/tree/main/aws/alb).

### Inputs

| Input               | Description                                                  | Example      |
| ------------------- | ------------------------------------------------------------ | ------------ |
| `INGRESS_NAME`      | The name of the ingress we are going to check the status of. | `api-public` |
| `INGRESS_NAMESPACE` | The namespace it lives in.                                   | `api`        |

### Outputs

Format: `json`

| Value       | Description                                           | Example                                               |
| ----------- | ----------------------------------------------------- | ----------------------------------------------------- |
| `status`    | The contents of the `Ingress.Status`.                 | `{"loadBalancer": {"ingress": [{"hostname":"..."}]}}` |
| `indicator` | A visual indicator of `success` for use in templates. | 🔴, 🟢                                                |

Note: success is determined by whether or not the ALB has been provisioned for
the `Ingress` and is available in its `Status` field

### Example Configuration

```toml
#:schema https://api.nuon.co/v1/general/config-schema?source=action
name    = "api-alb-healthcheck"
timeout = "1m0s"


[[triggers]]
type          = "cron"
cron_schedule = "0 */1 * * *"

[[triggers]]
type = "manual"

[[steps]]
name    = "alb-healthcheck"
command = "./alb-healthcheck"

[steps.public_repo]
repo      = "nuonco/actions"
branch    = "main"
directory = "aws"

[steps.env_vars]
INGRESS_NAME      = "api-public"
INGRESS_NAMESPACE = "api"
```
