#!/bin/bash

# KubeBlocks RBAC Permissions Summary Generator
# This script generates a concise summary of all RBAC permissions required by KubeBlocks

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
HELM_DIR="$PROJECT_ROOT/deploy/helm"

# Output file and temp file
OUTPUT_FILE="$PROJECT_ROOT/docs/rbac-summary.md"
TEMP_YAML="$PROJECT_ROOT/temp-helm-output.yaml"

echo -e "${BLUE}KubeBlocks RBAC Permissions Summary Generator${NC}"
echo "=============================================="

# Check dependencies
if ! command -v helm &> /dev/null; then
    echo -e "${RED}Error: helm is not installed${NC}"
    exit 1
fi

if ! command -v yq &> /dev/null; then
    echo -e "${RED}Error: yq is not installed${NC}"
    exit 1
fi

# Create output directory
mkdir -p "$(dirname "$OUTPUT_FILE")"

# Generate summary
echo -e "${YELLOW}Generating RBAC permissions summary...${NC}"

# Render templates once and save to temp file
echo -e "${YELLOW}Rendering Helm templates...${NC}"
cd "$HELM_DIR"
helm template kubeblocks . > "$TEMP_YAML"
cd "$PROJECT_ROOT"

cat > "$OUTPUT_FILE" << 'EOF'
# KubeBlocks RBAC Permissions Summary

This document provides a comprehensive summary of all RBAC permissions required by KubeBlocks.

## Overview

KubeBlocks requires extensive permissions across both standard Kubernetes resources and its custom resources to manage database clusters, backups, configurations, and operations.

EOF

echo "" >> "$OUTPUT_FILE"
echo "**Generated:** $(date)" >> "$OUTPUT_FILE"
echo "" >> "$OUTPUT_FILE"

# Function to generate resource permissions table using simpler yq operations
generate_permissions_table() {
    local title="$1"
    local filter="$2"

    echo "## $title" >> "$OUTPUT_FILE"
    echo "" >> "$OUTPUT_FILE"
    echo "| Resource | Permissions |" >> "$OUTPUT_FILE"
    echo "|----------|-------------|" >> "$OUTPUT_FILE"

    # Extract resource:verb pairs and use shell to group them
    local temp_data=$(yq eval "$filter" "$TEMP_YAML" 2>/dev/null | grep -v "^null$" | grep -v "^---$" | sort | uniq)

    if [[ -n "$temp_data" ]]; then
        echo "$temp_data" | awk -F: '
        {
            if ($1 in resources) {
                if (index(resources[$1], $2) == 0) {
                    resources[$1] = resources[$1] ", " $2
                }
            } else {
                resources[$1] = $2
            }
        }
        END {
            for (resource in resources) {
                if (resource != "" && resource != "null") {
                    print "| `" resource "` | " resources[resource] " |"
                }
            }
        }' | sort >> "$OUTPUT_FILE"
    fi

    echo "" >> "$OUTPUT_FILE"
}

# Core Kubernetes Resources
generate_permissions_table "Core Kubernetes Resources" '
  select(.kind == "ClusterRole" or .kind == "Role") |
  .rules[]? |
  select(.apiGroups[]? == "" or .apiGroups[]? == "apps" or .apiGroups[]? == "batch" or .apiGroups[]? == "coordination.k8s.io" or .apiGroups[]? == "storage.k8s.io" or .apiGroups[]? == "snapshot.storage.k8s.io") |
  .resources[]? as $resource |
  .verbs[]? as $verb |
  select($resource != null and $verb != null) |
  $resource + ":" + $verb
'

# RBAC Resources
rbac_count=$(yq eval '
  select(.kind == "ClusterRole" or .kind == "Role") |
  .rules[]? |
  select(.apiGroups[]? == "rbac.authorization.k8s.io") |
  .resources[]?
' "$TEMP_YAML" 2>/dev/null | grep -v "^null$" | wc -l | tr -d ' ')

if [[ "$rbac_count" -gt 0 ]]; then
    generate_permissions_table "RBAC Resources" '
      select(.kind == "ClusterRole" or .kind == "Role") |
      .rules[]? |
      select(.apiGroups[]? == "rbac.authorization.k8s.io") |
      .resources[]? as $resource |
      .verbs[]? as $verb |
      select($resource != null and $verb != null) |
      $resource + ":" + $verb
    '
fi

# Authentication & Authorization
auth_count=$(yq eval '
  select(.kind == "ClusterRole" or .kind == "Role") |
  .rules[]? |
  select(.apiGroups[]? == "authentication.k8s.io" or .apiGroups[]? == "authorization.k8s.io") |
  .resources[]?
' "$TEMP_YAML" 2>/dev/null | grep -v "^null$" | wc -l | tr -d ' ')

if [[ "$auth_count" -gt 0 ]]; then
    echo "## Authentication & Authorization" >> "$OUTPUT_FILE"
    echo "" >> "$OUTPUT_FILE"
    echo "| Resource | Permissions |" >> "$OUTPUT_FILE"
    echo "|----------|-------------|" >> "$OUTPUT_FILE"

    auth_data=$(yq eval '
      select(.kind == "ClusterRole" or .kind == "Role") |
      .rules[]? |
      select(.apiGroups[]? == "authentication.k8s.io" or .apiGroups[]? == "authorization.k8s.io") |
      (.apiGroups[]? + "/" + .resources[]?) as $resource |
      .verbs[]? as $verb |
      select($resource != null and $verb != null) |
      $resource + ":" + $verb
    ' "$TEMP_YAML" 2>/dev/null | grep -v "^null$" | grep -v "^---$" | sort | uniq)

    if [[ -n "$auth_data" ]]; then
        echo "$auth_data" | awk -F: '
        {
            if ($1 in resources) {
                if (index(resources[$1], $2) == 0) {
                    resources[$1] = resources[$1] ", " $2
                }
            } else {
                resources[$1] = $2
            }
        }
        END {
            for (resource in resources) {
                if (resource != "" && resource != "null") {
                    print "| `" resource "` | " resources[resource] " |"
                }
            }
        }' | sort >> "$OUTPUT_FILE"
    fi

    echo "" >> "$OUTPUT_FILE"
fi

# KubeBlocks Custom Resources
echo "## KubeBlocks Custom Resources" >> "$OUTPUT_FILE"
echo "" >> "$OUTPUT_FILE"

kb_api_groups=("apps.kubeblocks.io" "dataprotection.kubeblocks.io" "extensions.kubeblocks.io" "operations.kubeblocks.io" "parameters.kubeblocks.io" "workloads.kubeblocks.io" "experimental.kubeblocks.io" "trace.kubeblocks.io")

for api_group in "${kb_api_groups[@]}"; do
    # Check if this API group has any resources
    resources_count=$(yq eval "
      select(.kind == \"ClusterRole\" or .kind == \"Role\") |
      .rules[]? |
      select(.apiGroups[]? == \"$api_group\") |
      .resources[]? |
      select(. != null and (test(\"/status$|/finalizers$\") | not))
    " "$TEMP_YAML" 2>/dev/null | grep -v "^null$" | wc -l | tr -d ' ')

    if [[ "$resources_count" -gt 0 ]]; then
        echo "### $api_group" >> "$OUTPUT_FILE"
        echo "" >> "$OUTPUT_FILE"
        echo "| Resource | Permissions |" >> "$OUTPUT_FILE"
        echo "|----------|-------------|" >> "$OUTPUT_FILE"

        group_data=$(yq eval "
          select(.kind == \"ClusterRole\" or .kind == \"Role\") |
          .rules[]? |
          select(.apiGroups[]? == \"$api_group\") |
          .resources[]? as \$resource |
          .verbs[]? as \$verb |
          select(\$resource != null and \$verb != null and (\$resource | test(\"/status$|/finalizers$\") | not)) |
          \$resource + \":\" + \$verb
        " "$TEMP_YAML" 2>/dev/null | grep -v "^null$" | grep -v "^---$" | sort | uniq)

        if [[ -n "$group_data" ]]; then
            echo "$group_data" | awk -F: '
            {
                if ($1 in resources) {
                    if (index(resources[$1], $2) == 0) {
                        resources[$1] = resources[$1] ", " $2
                    }
                } else {
                    resources[$1] = $2
                }
            }
            END {
                for (resource in resources) {
                    if (resource != "" && resource != "null") {
                        print "| `" resource "` | " resources[resource] " |"
                    }
                }
            }' | sort >> "$OUTPUT_FILE"
        fi

        echo "" >> "$OUTPUT_FILE"
    fi
done

# Special permissions (Non-Resource URLs)
non_resource_count=$(yq eval '
  select(.kind == "ClusterRole" or .kind == "Role") |
  .rules[]? |
  select(.nonResourceURLs) |
  .nonResourceURLs[]?
' "$TEMP_YAML" 2>/dev/null | grep -v "^null$" | wc -l | tr -d ' ')

if [[ "$non_resource_count" -gt 0 ]]; then
    echo "## Special Permissions" >> "$OUTPUT_FILE"
    echo "" >> "$OUTPUT_FILE"
    echo "### Non-Resource URLs" >> "$OUTPUT_FILE"
    echo "" >> "$OUTPUT_FILE"
    echo "| URL | Permissions |" >> "$OUTPUT_FILE"
    echo "|-----|-------------|" >> "$OUTPUT_FILE"

    url_data=$(yq eval '
      select(.kind == "ClusterRole" or .kind == "Role") |
      .rules[]? |
      select(.nonResourceURLs) |
      .nonResourceURLs[]? as $url |
      .verbs[]? as $verb |
      select($url != null and $verb != null) |
      $url + ":" + $verb
    ' "$TEMP_YAML" 2>/dev/null | grep -v "^null$" | grep -v "^---$" | sort | uniq)

    if [[ -n "$url_data" ]]; then
        echo "$url_data" | awk -F: '
        {
            if ($1 in resources) {
                if (index(resources[$1], $2) == 0) {
                    resources[$1] = resources[$1] ", " $2
                }
            } else {
                resources[$1] = $2
            }
        }
        END {
            for (resource in resources) {
                if (resource != "" && resource != "null") {
                    print "| `" resource "` | " resources[resource] " |"
                }
            }
        }' | sort >> "$OUTPUT_FILE"
    fi

    echo "" >> "$OUTPUT_FILE"
fi

# Cleanup temp file
rm -f "$TEMP_YAML"

echo -e "${GREEN}RBAC permissions summary generated successfully!${NC}"
echo -e "Output file: $OUTPUT_FILE"