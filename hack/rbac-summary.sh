#!/bin/bash

# KubeBlocks RBAC Permissions Summary Generator
# This script generates a comprehensive summary of all RBAC permissions required by KubeBlocks
# under different configuration parameters

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

# Output file and temp files
OUTPUT_FILE="$PROJECT_ROOT/docs/rbac-summary.md"
TEMP_YAML_BASE="$PROJECT_ROOT/temp-helm-base.yaml"
TEMP_YAML_WEBHOOKS="$PROJECT_ROOT/temp-helm-webhooks.yaml"
TEMP_YAML_RBAC="$PROJECT_ROOT/temp-helm-rbac.yaml"
TEMP_YAML_BOTH="$PROJECT_ROOT/temp-helm-both.yaml"

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
echo -e "${YELLOW}Generating RBAC permissions summary for different configurations...${NC}"

# Render templates for different configurations
echo -e "${YELLOW}Rendering Helm templates for base configuration...${NC}"
cd "$HELM_DIR"
helm template kubeblocks . --set webhooks.conversionEnabled=false --set rbac.enabled=false > "$TEMP_YAML_BASE"

echo -e "${YELLOW}Rendering Helm templates with webhooks.conversionEnabled=true...${NC}"
helm template kubeblocks . --set webhooks.conversionEnabled=true --set rbac.enabled=false > "$TEMP_YAML_WEBHOOKS"

echo -e "${YELLOW}Rendering Helm templates with rbac.enabled=true...${NC}"
helm template kubeblocks . --set webhooks.conversionEnabled=false --set rbac.enabled=true > "$TEMP_YAML_RBAC"

echo -e "${YELLOW}Rendering Helm templates with both enabled...${NC}"
helm template kubeblocks . --set webhooks.conversionEnabled=true --set rbac.enabled=true > "$TEMP_YAML_BOTH"

cd "$PROJECT_ROOT"

# Function to extract and format permissions for a specific API group
extract_permissions() {
    local api_group="$1"
    local filter="$2"
    local temp_file="$3"

    local permissions=$(yq eval "$filter" "$temp_file" 2>/dev/null | grep -v "^null$" | grep -v "^---$" | sort | uniq)

    if [[ -n "$permissions" ]]; then
        echo "$permissions" | awk -F: '
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
                    print "* **" resource "**: " resources[resource]
                }
            }
        }' | sort
    fi
}

# Function to get all API groups from a template
get_api_groups() {
    local temp_file="$1"
    local exclude_kubeblocks="$2"

    if [[ "$exclude_kubeblocks" == "true" ]]; then
        yq eval '
          select(.kind == "ClusterRole" or .kind == "Role") |
          .rules[]? |
          .apiGroups[]? |
          select(. == "" or (test("kubeblocks") | not))
        ' "$temp_file" 2>/dev/null | grep -v "^null$" | sort | uniq
    else
        yq eval '
          select(.kind == "ClusterRole" or .kind == "Role") |
          .rules[]? |
          .apiGroups[]? |
          select(. != null and test("kubeblocks"))
        ' "$temp_file" 2>/dev/null | grep -v "^null$" | sort | uniq
    fi
}

# Function to generate permissions for a specific configuration
generate_permissions_section() {
    local temp_file="$1"
    local section_title="$2"
    local output_file="$3"

    echo "### $section_title" >> "$output_file"
    echo "" >> "$output_file"

    # Get all unique API groups from Kubernetes (excluding KubeBlocks groups)
    local all_k8s_api_groups=$(get_api_groups "$temp_file" "true")

    # Process each API group
    while IFS= read -r api_group; do
        # Process all API groups including empty string (core API group)
        if [[ -n "$api_group" ]] || [[ "$api_group" == "" ]]; then
            # Check if this API group has any resources
            resources_count=$(yq eval "
              select(.kind == \"ClusterRole\" or .kind == \"Role\") |
              .rules[]? |
              select(.apiGroups[]? == \"$api_group\") |
              .resources[]? |
              select(. != null)
            " "$temp_file" 2>/dev/null | grep -v "^null$" | wc -l | tr -d ' ')

            if [[ "$resources_count" -gt 0 ]]; then
                # Display API group name
                if [[ "$api_group" == "" ]]; then
                    echo "#### Core API Group" >> "$output_file"
                else
                    echo "#### $api_group" >> "$output_file"
                fi
                echo "" >> "$output_file"

                # Extract permissions for this API group
                api_permissions=$(extract_permissions "$api_group" "
                  select(.kind == \"ClusterRole\" or .kind == \"Role\") |
                  .rules[]? |
                  select(.apiGroups[]? == \"$api_group\") |
                  .resources[]? as \$resource |
                  .verbs[]? as \$verb |
                  select(\$resource != null and \$verb != null) |
                  \$resource + \":\" + \$verb
                " "$temp_file")

                if [[ -n "$api_permissions" ]]; then
                    echo "$api_permissions" >> "$output_file"
                    echo "" >> "$output_file"
                fi
            fi
        fi
    done <<< "$all_k8s_api_groups"

    # Non-Resource URLs (if any)
    non_resource_count=$(yq eval '
      select(.kind == "ClusterRole" or .kind == "Role") |
      .rules[]? |
      select(.nonResourceURLs) |
      .nonResourceURLs[]?
    ' "$temp_file" 2>/dev/null | grep -v "^null$" | wc -l | tr -d ' ')

    if [[ "$non_resource_count" -gt 0 ]]; then
        echo "#### Non-Resource URLs" >> "$output_file"
        echo "" >> "$output_file"

        non_resource_permissions=$(yq eval '
          select(.kind == "ClusterRole" or .kind == "Role") |
          .rules[]? |
          select(.nonResourceURLs) |
          .nonResourceURLs[]? as $url |
          .verbs[]? as $verb |
          select($url != null and $verb != null) |
          $url + ":" + $verb
        ' "$temp_file" 2>/dev/null | grep -v "^null$" | grep -v "^---$" | sort | uniq)

        if [[ -n "$non_resource_permissions" ]]; then
            echo "$non_resource_permissions" | awk -F: '
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
                        print "* **" resource "**: " resources[resource]
                    }
                }
            }' | sort >> "$output_file"
            echo "" >> "$output_file"
        fi
    fi

    # KubeBlocks Custom Resources
    local kb_api_groups_raw=$(get_api_groups "$temp_file" "false")

    # Convert to array
    local kb_api_groups=()
    while IFS= read -r group; do
        if [[ -n "$group" ]]; then
            kb_api_groups+=("$group")
        fi
    done <<< "$kb_api_groups_raw"

    if [[ ${#kb_api_groups[@]} -gt 0 ]]; then
        echo "#### KubeBlocks Custom Resources" >> "$output_file"
        echo "" >> "$output_file"

        for api_group in "${kb_api_groups[@]}"; do
            # Check if this API group has any resources
            resources_count=$(yq eval "
              select(.kind == \"ClusterRole\" or .kind == \"Role\") |
              .rules[]? |
              select(.apiGroups[]? == \"$api_group\") |
              .resources[]? |
              select(. != null)
            " "$temp_file" 2>/dev/null | grep -v "^null$" | wc -l | tr -d ' ')

            if [[ "$resources_count" -gt 0 ]]; then
                echo "##### $api_group" >> "$output_file"
                echo "" >> "$output_file"

                # Extract permissions for this API group
                kb_permissions=$(extract_permissions "$api_group" "
                  select(.kind == \"ClusterRole\" or .kind == \"Role\") |
                  .rules[]? |
                  select(.apiGroups[]? == \"$api_group\") |
                  .resources[]? as \$resource |
                  .verbs[]? as \$verb |
                  select(\$resource != null and \$verb != null) |
                  \$resource + \":\" + \$verb
                " "$temp_file")

                if [[ -n "$kb_permissions" ]]; then
                    echo "$kb_permissions" >> "$output_file"
                    echo "" >> "$output_file"
                fi
            fi
        done
    fi
}

# Function to compare permissions and show differences
compare_permissions() {
    local base_file="$1"
    local compare_file="$2"
    local section_title="$3"
    local output_file="$4"

    echo "### $section_title" >> "$output_file"
    echo "" >> "$output_file"

    # Get all permissions from both files
    base_permissions=$(yq eval '
      select(.kind == "ClusterRole" or .kind == "Role") |
      .rules[]? |
      (.apiGroups[]? + ":" + .resources[]? + ":" + .verbs[]?) |
      select(. != null)
    ' "$base_file" 2>/dev/null | grep -v "^null$" | sort | uniq)

    compare_permissions=$(yq eval '
      select(.kind == "ClusterRole" or .kind == "Role") |
      .rules[]? |
      (.apiGroups[]? + ":" + .resources[]? + ":" + .verbs[]?) |
      select(. != null)
    ' "$compare_file" 2>/dev/null | grep -v "^null$" | sort | uniq)

    # Find new permissions
    new_permissions=$(comm -13 <(echo "$base_permissions" | sort) <(echo "$compare_permissions" | sort))

    if [[ -n "$new_permissions" ]]; then
        echo "**Additional permissions required:**" >> "$output_file"
        echo "" >> "$output_file"

        # Group by API group and sort properly
        temp_output=$(mktemp)
        echo "$new_permissions" | awk -F: '
        {
            api_group = $1
            resource = $2
            verb = $3
            if (api_group == "") api_group = "Core API Group"
            key = api_group ":" resource
            if (key in perms) {
                if (index(perms[key], verb) == 0) {
                    perms[key] = perms[key] ", " verb
                }
            } else {
                perms[key] = verb
            }
        }
        END {
            for (key in perms) {
                split(key, parts, ":")
                group = parts[1]
                resource = parts[2]
                print group "\t" resource "\t" perms[key]
            }
        }' | sort | awk -F'\t' '
        {
            group = $1
            resource = $2
            permissions = $3
            if (group != current_group) {
                if (current_group != "") print ""
                print "#### " group
                print ""
                current_group = group
            }
            print "* **" resource "**: " permissions
        }' >> "$output_file"
        echo "" >> "$output_file"
    else
        echo "**No additional permissions required.**" >> "$output_file"
        echo "" >> "$output_file"
    fi
}

# Start generating the output file
cat > "$OUTPUT_FILE" << 'EOF'
# KubeBlocks Operator RBAC Permissions

KubeBlocks operator requires different permissions based on configuration parameters.
EOF

echo "" >> "$OUTPUT_FILE"
echo "**Generated:** $(date)" >> "$OUTPUT_FILE"
echo "" >> "$OUTPUT_FILE"

# Generate base configuration
echo "## Base Configuration" >> "$OUTPUT_FILE"
echo "" >> "$OUTPUT_FILE"
echo "**Configuration:** \`webhooks.conversionEnabled=false\` and \`rbac.enabled=false\`" >> "$OUTPUT_FILE"
echo "" >> "$OUTPUT_FILE"

generate_permissions_section "$TEMP_YAML_BASE" "Kubernetes Resource Permissions" "$OUTPUT_FILE"

# Generate comparison sections
echo "## Additional Permissions for Different Configurations" >> "$OUTPUT_FILE"
echo "" >> "$OUTPUT_FILE"

compare_permissions "$TEMP_YAML_BASE" "$TEMP_YAML_WEBHOOKS" "webhooks.conversionEnabled=true" "$OUTPUT_FILE"
compare_permissions "$TEMP_YAML_BASE" "$TEMP_YAML_RBAC" "rbac.enabled=true" "$OUTPUT_FILE"

# Cleanup temp files
rm -f "$TEMP_YAML_BASE" "$TEMP_YAML_WEBHOOKS" "$TEMP_YAML_RBAC" "$TEMP_YAML_BOTH"

echo -e "${GREEN}RBAC permissions summary generated successfully!${NC}"
echo -e "Output file: $OUTPUT_FILE"