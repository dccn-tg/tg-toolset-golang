#!/bin/bash

API_URL="https://131.174.44.94/api"
ADMIN="roadmin"
PASS=""
SVM="atreides"

# curl command prefix
CURL="curl -k"

# Print usage message and document.
function usage() {
    echo "Usage: $1 <new|get|qot|del> <projectID> {<sizeGb>}" 1>&2
    cat << EOF >&2

This script is a demo implementation of managing project datasets
using the ONTAP management APIs. It requires "curl" and
"jq".

API documentation: https://library.netapp.com/ecmdocs/ECMLP2856304/html/index.html 

Operations:
    new: creates new dataset for a project.
    get: retrieves information of the project dataset.
    qot: configures quota of the project dataset.
    del: deletes dataset of a project.

EOF
}

# Convert projectID to volume name using a convention
function projectVolName() {
    echo "project_${1}" | sed 's/\./_/g'
}

# Get UUID
function getUUID() {

    name=$1
    api_ns=$2

    ${CURL} -X GET -u ${ADMIN}:${PASS} "${API_URL}/${api_ns}" | \
    jq --arg name "$name" \
       '.records[] | select(.name == $name) | .uuid' | \
    sed 's/"//g'
}

# Get Object attributes by UUID in a generic way.
function getObjectByUUID() {

    uuid=$1
    api_ns=$2

    filter=".|.$(echo ${@:3} | sed 's/ /,./g')"

    ${CURL} -X GET -u ${ADMIN}:${PASS} "${API_URL}/${api_ns}/${uuid}" | \
    jq ${filter}
}

# Get Object attributes by name in a generic way.
function getObjectByName() {
    name=$1
    api_ns=$2

    uuid=$(getUUID $name $api_ns)

    [ "" == "$uuid" ] && echo "object not found: $name" >&2 && return 1
   
    getObjectByUUID $uuid $api_ns ${@:3}
}

# Creates a new volume with given name and size.
function newVolume() {

    name=$1
    quota=$( echo "$2 * 1024 * 1024 * 1024" | bc )
 
    # check if volume exists.
    getObjectByName $name '/storage/volumes' 'uuid' >/dev/null 2>&1
    [ $? -eq 0 ] && echo "volume already exists: $name" >&2 && return 1

    # determine which aggregate to use.
    aggr=""
    size=0
    for id in $(getObjectByName $SVM '/svm/svms' 'aggregates[].uuid' 2>/dev/null | sed 's/"//g'); do

        avail=$(getObjectByUUID $id "/storage/aggregates" space.block_storage.available 2>/dev/null)

        [ $avail -gt $quota ] && [ $avail -gt $size ] && aggr=$id && size=$avail
    done
    
    # create new volume on aggregate
    out=$( ${CURL} -X POST -u ${ADMIN}:${PASS} \
            -H 'content-type: application/json' \
            -d $(dataVolume $name $quota $aggr) \
            ${API_URL}/storage/volumes )

    [ "$(echo $out | jq '.error' )" != "" ] &&
        echo "fail to create volume: $name" >&2 &&
        echo $out | jq && return 1

    echo $out | jq
}

# Compose POST data for creating new volume
function dataVolume() {

    name=$1
    size=$2
    aggr=$3

    #'.|.name=$name|.size=($size|tonumber)|.aggregates+=[{.uuid=$aggr}]' << EOF
    jq --arg name "$name" \
       --arg size "$size" \
       --arg aggr "$aggr" \
       --arg svm "$SVM" \
       -c -M \
       '.|.name=$name|.size=($size|tonumber)|.aggregates+=[{"uuid": $aggr}]|.svm.name=$svm' << EOF
{
  "svm": {},
  "state": "online",
  "style": "flexvol",
  "qos": {
    "policy": {
      "max_throughput_iops": "6000"
    }
  },
  "aggregates": []
}

EOF

}

# Get SVM
#function getSvm() {
#
#    uuid=$(getUUID $SVM "/svm/svms")
#
#    [ "" == "$uuid" ] && echo "SVM not found: $SVM" >&2 && return 1
#
#    filter=".|.$(echo $@ | sed 's/ /,./g')"
#
#    ${CURL} -X GET -u ${ADMIN}:${PASS} "${API_URL}/svm/svms/${uuid}" | \
#    jq ${filter}
#}

## main program
[ $# -lt 2 ] && usage $0 && exit 1
ops=$1
projectID=$2

echo -n "Password for API user ($ADMIN): "
read -s PASS
echo

case $ops in
new)

    [ $# -lt 3 ] && usage $0 && exit 1
    sizeGb=$3
    
    newVolume $(projectVolName $projectID) $sizeGb || exit 1
    ;;
get)
    getObjectByName $(projectVolName $projectID) "/storage/volumes" || exit 1
    ;;
del)
    ;;
qot)
    [ $# -lt 3 ] && usage $0 && exit 1
    sizeGb=$3
    ;;
*)
    echo "unknown operation: $ops" >&2 && exit 1
    ;;
esac

exit 0
