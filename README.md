# env-sync

This Go utility allows you to transfer env variables from one project to another; it uses your PAT and API to do that.

## Usage

Clone the repository, build binary:

```bash
go build -o gitlab-env-sync
```

Navigate to your GitLab profile, create access token to use.

Run binary

```bash
./gitlab-env-sync --gitlab-url "" --token "" --source "" --target "" --dry-run
```

This will make a dry-run and output 'action' into file for you to inspect; when you have verified that everything is OK, remove the dry-run flag.

