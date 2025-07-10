#!/bin/bash

REPO_URL="https://github.com/apecloud/kubeblocks-addons.git"
EXAMPLES_DIR="examples"
CLONE_DIR="kubeblocks-addons-tmp"
KEEP_DIRS=(mysql postgresql redis mongodb kafka milvus qdrant rabbitmq elasticsearch)

# Clean up any previous temp directory
rm -rf "$CLONE_DIR"
rm -rf "$EXAMPLES_DIR"

# Clone with sparse-checkout
git clone --filter=blob:none --no-checkout "$REPO_URL" "$CLONE_DIR"
cd "$CLONE_DIR"
git sparse-checkout init --cone
git sparse-checkout set "$EXAMPLES_DIR"
git checkout


mv "$EXAMPLES_DIR" ../
# Go back and remove the cloned repo
cd ..
rm -rf "$CLONE_DIR"

# #!/bin/bash
# ## remove folder not in $KEEP_DIRS

for dir in "$EXAMPLES_DIR"/*; do
  # Check if $dir is a directory AND if it's NOT in the KEEP_DIRS array
  # -d "$dir" tests if it's a directory
  # The pattern match checks if the directory name (with spaces around it) is NOT found in KEEP_DIRS array
  if [ -d "$dir" ] && [[ ! " ${KEEP_DIRS[*]} " =~ " $(basename "$dir") " ]]; then
    rm -rf "$dir"
  fi
done

echo "Done! Only selected example directories have been copied to $(pwd)"