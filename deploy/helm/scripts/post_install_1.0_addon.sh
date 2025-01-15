#!/bin/bash
release=
namespace=
addon=

for i in "$@"; do
  case $i in
    --release-name=*)
      release="${i#*=}"
      shift
      ;;
    --namespace=*)
      namespace="${i#*=}"
      shift
      ;;
    *)
      echo "Unknown option $i"
      exit 1
      ;;
  esac
done
if [ "$release" == "" ] || [ "$namespace" == "" ]; then
  echo "--release-name, --namespace"
  exit 1
fi
echo "release: $release, namespace: $namespace"

helm get manifest -n $namespace $release | kubectl apply -f -

