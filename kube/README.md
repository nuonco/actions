# Kubernetes Actions

## [List Nodes](./list-nodes)

Get a list of all of the nodes running in a k8s cluster.

### Inputs

This action takes no inputs.

### Outputs

Format: `json`

| Value       | Description                            | Example           |
| ----------- | -------------------------------------- | ----------------- |
| `exit_code` | The exit code of the `kubectl` command | 0, 1, 127         |
| `content`   | The `stdout` of the `kubectl` command  | see exmaple below |

**Example Content Output**

```txt
Name                                           Status   AZ           Size           Create
ip-10-128-128-156.us-east-2.compute.internal   Ready    us-east-2a   t3a.medium     2025-02-14T02:45:25Z
ip-10-128-129-185.us-east-2.compute.internal   Ready    us-east-2b   t3a.medium     2025-02-14T02:45:25Z
ip-10-128-130-6.us-east-2.compute.internal     Ready    us-east-2c   inf2.8xlarge   2025-02-14T02:45:25Z
ip-10-128-128-78.us-east-2.compute.internal    Ready    us-east-2a   inf2.8xlarge   2025-02-14T02:45:26Z
ip-10-128-130-232.us-east-2.compute.internal   Ready    us-east-2c   t3a.medium     2025-02-14T02:45:28Z
```

### Example Configuration

```toml
#:schema https://api.nuon.co/v1/general/config-schema?source=action
name    = "list-nodes"
timeout = "0m30s"


[[triggers]]
type          = "cron"
cron_schedule = "*/15 */1 * * *"

[[triggers]]
type = "manual"

[[steps]]
name    = "kube-list-nodes"
command = "./list-nodes"

[steps.public_repo]
repo      = "nuonco/actions"
branch    = "main"
directory = "kube"
```
