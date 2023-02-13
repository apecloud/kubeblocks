#!/usr/bin/env bash

main() {
    local EXIT_STATUS=0
    images=`helm get manifest kubeblocks | grep 'image:' | awk '{print $2}' | sort -u | sed 's/"//g'`
    for image in $images; do
        if [[ "$image" == "docker.io/apecloud/kubeblocks"* ]]; then
            continue
        fi
        echo "check image:$image"
        check_image $image
    done
    echo $EXIT_STATUS
    exit $EXIT_STATUS
}

check_image() {
    image=$1
    if [[  "$image" == "quay.io/"* ]]; then
        echo "Use the quay.io repository image:$image, which should be replaced."
        EXIT_STATUS=1
    elif [[  "$image" == "docker.io/apecloud/"* ]]; then
        check_image_exists $image
    fi
}

check_image_exists() {
    image=$1
    image_url=${image/:/\/tags\/}
    image_url=${image_url/docker.io/https://hub.docker.com/v2/repositories}
    exists_flag=`curl -s $image_url | grep digest`
    if [[ -z "$exists_flag" ]]; then
        echo "$image is not exists."
        EXIT_STATUS=1
    fi
}

main "$@"
