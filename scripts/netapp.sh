#!/bin/bash

API_URL="https://131.174.44.94/api"
[ -z $ADMIN ] && ADMIN="roadmin"
PASS=""
SVM="atreides"

# curl command prefix
CURL="curl -k"

[ -z $EXPORT_POLICY ] && EXPORT_POLICY="dccn-projects"
[ -z $PATH_PROJECT ] && PATH_PROJECT="/project"
[ -z $UID_PROJECT ] && UID_PROJECT="1010"
[ -z $GID_PROJECT ] && GID_PROJECT="1010"

# Print usage message and document.
function usage() {
    echo "Usage: $1 <new|get|qot|del> <projectID> [<sizeGb>]" 1>&2
    cat << EOF >&2

This script is a demo implementation of managing project flexvol/qtree
using the ONTAP management APIs. It requires "curl" and "jq".

API documentation: https://library.netapp.com/ecmdocs/ECMLP2856304/html/index.html 

Environment variables:
            ADMIN: username for accessing the API server.
    EXPORT_POLICY: NAS export policy name
     PATH_PROJECT: NAS export path
      UID_PROJECT: numerical uid of the user "project"
      GID_PROJECT: numerical gid of the group "project_g"

Operations:
    new: creates new space for a project or a user home.
    get: retrieves information of the space.
    qot: configures quota of the project/user home space.
    del: deletes space of a project or a user home.

EOF
}

# Convert projectID to volume name using a convention
function projectVolName() {
    echo "project_${1}" | sed 's/\./_/g'
}

# Convert projectID to NAS path using a convention
function projectNasPath() {
    echo "${PATH_PROJECT}/${1}"
}

# Get UUID
function getUUID() {

    name=$1
    api_ns=$2

    case $(basename $api_ns) in
    qtree)
        ${CURL} -X GET -u ${ADMIN}:${PASS} "${API_URL}/${api_ns}" | \
        jq --arg name "$name" \
           '.records[] | select(.name == $name) | .id' | \
        sed 's/"//g'
        ;;
    *)
        ${CURL} -X GET -u ${ADMIN}:${PASS} "${API_URL}/${api_ns}" | \
        jq --arg name "$name" \
           '.records[] | select(.name == $name) | .uuid' | \
        sed 's/"//g'
        ;;
    esac
}

# Get ID, in some cases, objects are 
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

# Creates a new qtree with given qtree path
function newQtree() {
    name=$1
    volname=$2
    quota=$( echo "$3 * 1024 * 1024 * 1024" | bc )

    # first get volume uuid
    voluuid=$(getObjectByName $name '/storage/volumes' 'uuid' >&2)
    [ -z $voluuid ] && echo "volume doesn't exist: $volname" >&2 && return 1

    # check if tree already exists
    getUUID $name '/storage/qtrees' >/dev/null 2>&1 &&
        echo "qtree already exists: $name" >&2 && return 1

    # create new qtree
    out=$( ${CURL} -X POST -u ${ADMIN}:${PASS} \
            -H 'content-type: application/json' \
            -d $(dataQtree $name $volname) \
            ${API_URL}/storage/qtrees )

    [ "$(echo $out | jq '.error' )" != "" ] &&
        echo "fail to create qtree: $name" >&2 &&
        echo $out | jq && return 1

    echo $out | jq
}

# Creates a new volume with given name and size.
function newVolume() {

    name=$1
    quota=$( echo "$2 * 1024 * 1024 * 1024" | bc )
    path=$3
 
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
            -d $(dataProjectVolume $name $quota $aggr $path) \
            ${API_URL}/storage/volumes )

    [ "$(echo $out | jq '.error' )" != "" ] &&
        echo "fail to create volume: $name" >&2 &&
        echo $out | jq && return 1

    echo $out | jq
}

# Set volume size
function resizeVolume() {
    name=$1
    quota=$( echo "$2 * 1024 * 1024 * 1024" | bc )

    # check if volume exists.
    uuid=$(getUUID $name '/storage/volumes' 2>/dev/null)
    [ "$uuid" == "" ] && echo "volume does not exists: $name" >&2 && return 1

    # resizing volume 
    out=$( ${CURL} -X PATCH -u ${ADMIN}:${PASS} \
            -H 'content-type: application/json' \
            -d $(dataResizeVolume $name $quota) \
            ${API_URL}/storage/volumes/${uuid} )

    [ "$(echo $out | jq '.error' )" != "" ] &&
        echo "fail to resize volume: $name" >&2 &&
        echo $out | jq && return 1

    echo $out | jq
}

# Compose POST data for creating new qtree
function dataQtree() {

    name=$1
    volname=$2

    jq --arg name "$name" \
       --arg volname "$volname" \
       --arg svm "$SVM" \
       --arg pexport "$EXPORT_POLICY" \
       -c -M \
       '.|.name=$name|.volume.name=$volname|.svm.name=$svm|.export_policy.name=$pexport' << EOF
{
  "svm": {},
  "volume": {},
  "export_policy": {},
  "security_style": "unix",
  "unix_permissions": "0700"
}

EOF
}

# Compose POST data for volume resizing
function dataResizeVolume() {

    name=$1
    size=$2

    jq --arg name "$name" \
       --arg size "$size" \
       -c -M \
       '.|.name=$name|.size=($size|tonumber)' << EOF
{}

EOF

}

# Compose POST data for creating new volume
function dataProjectVolume() {

    name=$1
    size=$2
    aggr=$3
    path=$4

    jq --arg svm  "$SVM" \
       --arg name "$name" \
       --arg size "$size" \
       --arg aggr "$aggr" \
       --arg path "$path" \
       --arg pexport "$EXPORT_POLICY" \
       --arg uid "$UID_PROJECT" \
       --arg gid "$GID_PROJECT" \
       -c -M \
       '.|.name=$name|.size=($size|tonumber)|.aggregates+=[{"uuid": $aggr}]|.svm.name=$svm|.nas.path=$path|.nas.uid=($uid|tonumber)|.nas.gid=($gid|tonumber)|.nas.export_policy.name=$pexport' << EOF
{
  "svm": {},
  "state": "online",
  "style": "flexvol",
  "qos": {
    "policy": {
      "max_throughput_iops": "6000"
    }
  },
  "aggregates": [],
  "autosize": {
    "mode": "off"
  },
  "nas": {
    "security_style": "unix",
    "unix_permissions": "0750",
    "export_policy": {}
  }
}

EOF

}

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

    volName=$(projectVolName $projectID)
    nasPath=$(projectNasPath $projectID)
 
    newVolume $volName $sizeGb $nasPath || exit 1
    ;;
get)
    getObjectByName $(projectVolName $projectID) "/storage/volumes" || exit 1
    ;;
del)
    ;;
qot)
    [ $# -lt 3 ] && usage $0 && exit 1
    sizeGb=$3
    volName=$(projectVolName $projectID)

    resizeVolume $volName $sizeGb || exit 1
    ;;
*)
    echo "unknown operation: $ops" >&2 && exit 1
    ;;
esac

exit 0
