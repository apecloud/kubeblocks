#!/usr/bin/env bash

main() {
    local EXIT_STATUS=0
    local IMAGE_ARRAY=()
    get_images
    for image in ${IMAGE_ARRAY[@]}; do
        echo "check image:$image"
        check_image $image
    done
    echo $EXIT_STATUS
    exit $EXIT_STATUS
}

get_images() {
    images=`helm get manifest kubeblocks | grep 'image:' | awk '{print $2}' | sed 's/\"//g'`
    for image in $images; do
        if [[ "$image" == "docker.io/apecloud/kubeblocks"* ]]; then
            continue
        fi
        duplicate_flag=false
        for arr in ${IMAGE_ARRAY[@]}; do
            if [[ "$arr" == "$image" ]]; then
                duplicate_flag=true
                break
            fi
        done
        if [[ "$duplicate_flag" == "false" ]]; then
            IMAGE_ARRAY[${#IMAGE_ARRAY[@]}]=$image
        fi
    done
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
