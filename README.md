# terraformlib

I got tired of writing the same terraform commands over and over, so I made this.

## What you get

- Merges config and manifest YAML files
- Generates terraform.json automatically
- Runs terraform commands for you
- Keeps files organized in .terraform/
- Auto-installs missing dependencies

## Usage

```bash
# Plan your changes
terraformlib plan -c config.yaml -m manifest.yaml -s terraform-dir

# Apply them (no need to specify files again)
terraformlib apply -s terraform-dir

# Clean up when done
terraformlib destroy -s terraform-dir
```

## Install

```bash
go install .
```

## Example

```bash
terraformlib plan -c shared-config/deploy/k8s-cluster/dev/terraform/terraform.yaml -m shared-manifest/deploy/k8s-cluster/dev/terraform/terraform.yaml -s tf-root-k8s-cluster
```