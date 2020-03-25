#!/bin/bash

API_URL="https://freenas.dccn.nl/api/v2.0"
DATASET_PREFIX="zpool001/project"
ZFS_PATH_PREFIX="/mnt/${DATASET_PREFIX}"
ADMIN="root"
PASS=""

# Print usage message and document.
function usage() {
    echo "Usage: $1 <new|get|qot|nfs|del> <projectID> {<sizeGb>}" 1>&2
    cat << EOF >&2

This script is a demo implementation of managing project datasets
using the FreeNAS/TrueNAS management APIs. It requires "curl" and
"jq".

API documentation: https://freenas.dccn.nl/api/docs

Operations:
    new: creates new dataset for a project.
    get: retrieves information of the project dataset.
    qot: configures quota of the project dataset.
    nfs: enables NFS share for the project dataset.
    del: deletes dataset of a project.

EOF
}

# Get dataset information with selected attributes.
function getDataset() {
    filter=".|.$(echo ${@:2} | sed 's/ /,./g')"
    id=$(echo "${DATASET_PREFIX}/$1" | sed 's|/|%2F|g')
    curl -X GET -u ${ADMIN}:${PASS} "${API_URL}/pool/dataset/id/${id}" | jq ${filter}
}

# Get NFS sharing for a dataset with selected attributes.
#   - returns 1 if the dataset sharing is not found.
function getDatasetSharingNFS() {

    path=${ZFS_PATH_PREFIX}/$1

    filter=".|.$(echo ${@:2} | sed 's/ /,./g')"

    share=$( curl -X GET -u ${ADMIN}:${PASS} "${API_URL}/sharing/nfs" | \
             jq --arg path "${path}" '.[] | select(.paths == [$path]) | .' )

    [ "" == "$share" ] && return 1 || (echo $share | jq $filter)
}

# Set dataset permission
function setDatasetPermission() {
    pid=$1

    id=$(getDataset $pid id 2>/dev/null)
    [ $? -ne 0 ] && echo "dataset doesn't exist: $pid" >&2 && return 1 

    id=$(echo $id | sed 's/"//g' | sed 's|/|%2F|g')

    curl -X POST -H 'content-type:application/json' \
         -d "$(dataPermission)" \
         -u ${ADMIN}:${PASS} "${API_URL}/pool/dataset/id/${id}/permission" &&
    echo "dataset permission set: $pid" > /dev/tty ||
    (echo "fail to set dataset permission: $pid" >&2 && return 1)
}

# Set dataset quota 
function setDatasetQuota() {
    pid=$1
    quotaGb=$2

    id=$(getDataset $pid id 2>/dev/null)
    [ $? -ne 0 ] && echo "dataset doesn't exist: $pid" >&2 && return 1 

    id=$(echo $id | sed 's/"//g' | sed 's|/|%2F|g')

    curl -X PUT -H 'content-type:application/json' \
         -d "$(dataQuotaUpdate $quotaGb)" \
         -u ${ADMIN}:${PASS} "${API_URL}/pool/dataset/id/${id}" &&
    echo "dataset quota set: $pid" > /dev/tty ||
    (echo "fail to set dataset quota: $pid" >&2 && return 1)
}

# Set dataset NFS sharing
#  - because the project storage is created as a separate dataset
function setDatasetSharingNFS() {
    pid=$1

    id=$(getDataset $pid id 2>/dev/null)
    [ $? -ne 0 ] && echo "dataset doesn't exist: $pid" >&2 && return 1 

    id=$(echo $id | sed 's/"//g' | sed 's|/|%2F|g')

    getDatasetSharingNFS $pid >/dev/null 2>&1
    [ $? -eq 0 ] && echo "NFS share exists: $pid" >&2 && return 0

    curl -X POST -H 'content-type:application/json' \
         -d "$(dataSharingNFS $pid)" \
         -u ${ADMIN}:${PASS} "${API_URL}/sharing/nfs" &&
    echo "dataset NFS share set: $pid" > /dev/tty ||
    (echo "fail to set dataset NFS share: $pid" >&2 && return 1)
}

# Deletes dataset NFS sharing
function delDatasetSharingNFS() {
    pid=$1

    id=$(getDatasetSharingNFS $pid id 2>/dev/null)
    [ $? -ne 0 ] && echo "NFS share doesn't exist: $pid" >&2 && return 0

    curl -X DELETE -u ${ADMIN}:${PASS} "${API_URL}/sharing/nfs/id/${id}" &&
    echo "dataset NFS share deleted: $pid" > /dev/tty ||
    (echo "fail to delete dataset NFS share: $pid" >&2 && return 1)
}

# Delete dataset
function delDataset() {
    pid=$1

    id=$(getDataset $pid id 2>/dev/null)
    [ $? -ne 0 ] && echo "dataset doesn't exist: $pid" >&2 && return 1
   
    id=$(echo $id | sed 's/"//g' | sed 's|/|%2F|g')

    curl -X DELETE -u ${ADMIN}:${PASS} "${API_URL}/pool/dataset/id/${id}" &&
    echo "dataset deleted: $pid" > /dev/tty ||
    (echo "fail to delete dataset: $pid" >&2 && return 1)
}

# Create dataset information
function newDataset() {

    pid=$1
    sgb=$2

    getDataset $pid >/dev/null 2>&1

    [ $? -eq 0 ] && echo "dataset already exists: $pid" >&2 && return 1

    curl -X POST -H 'content-type:application/json' \
         -d "$(dataDataset $pid $sgb)" \
         -u ${ADMIN}:${PASS} "${API_URL}/pool/dataset" &&
    echo "dataset created: $pid" > /dev/tty ||
    (echo "fail to create dataset: $pid" >&2 && return 1)
}

# Compose POST data for setting NFS sharing on dataset
function dataSharingNFS() {

    jq --arg path "${ZFS_PATH_PREFIX}/$1" \
       -c -M \
       '. | .paths += [$path]' << EOF
{
  "alldirs": false,
  "ro": false,
  "quiet": false,
  "maproot_user": "root",
  "maproot_group": "project_g",
  "mapall_user": null,
  "mapall_group": null,
  "security": ["SYS"],
  "paths": [],
  "networks": [
   "131.174.44.0/24",
   "131.174.45.0/24"
  ]
}

EOF

}

# Compose POST data for setting dataset permission
function dataPermission() {
    jq -c -M << EOF
{
  "user": "project",
  "group": "project_g",
  "mode": "0750",
  "acl": "UNIX",
  "recursive": true
}

EOF

}

# Compose POST data for setting dataset quota 
function dataQuotaUpdate() {

    size=$(echo "$1 * 1024 * 1024 * 1024" | bc)

    jq --arg size "$size" \
       -c -M \
       '. | .refquota=($size|tonumber)' << EOF
{}

EOF

}

# Compose POST data for creating dataset 
function dataDataset() {

    cmt="project $1"
    size=$(echo "$2 * 1024 * 1024 * 1024" | bc)

    jq --arg name "${DATASET_PREFIX}/$1" \
       --arg size "$size" \
       --arg cmt "$cmt" \
       -c -M \
       '. | .name=$name | .refquota=($size|tonumber) | .comments=$cmt' << EOF
{
  "type": "FILESYSTEM",
  "sync": "STANDARD",
  "compression": "LZ4",
  "atime": "ON",
  "exec": "ON",
  "reservation": 0,
  "refreservation": 0,
  "copies": 1,
  "snapdir": "HIDDEN",
  "deduplication": "OFF",
  "readonly": "OFF",
  "recordsize": "128K",
  "casesensitivity": "SENSITIVE",
  "share_type": "UNIX"
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
    
    newDataset $projectID $sizeGb &&
        setDatasetPermission $projectID &&
        setDatasetSharingNFS $projectID || exit 1
    ;;
get)
    getDataset $projectID || exit 1
    ;;
del)
    delDatasetSharingNFS $projectID &&
        delDataset $projectID || exit 1
    ;;
qot)
    [ $# -lt 3 ] && usage $0 && exit 1
    sizeGb=$3

    setDatasetQuota $projectID $sizeGb || exit 1
    ;;
nfs)
    setDatasetSharingNFS $projectID || exit 1
    ;;
*)
    echo "unknown operation: $ops" >&2 && exit 1
    ;;
esac

exit 0
