#!/bin/bash

[ -z $API_HOST ] && API_HOST="https://irulan-mgmt.dccn.nl"
[ -z $API_USER ] && API_USER="roadmin"
[ -z $SVM ] && SVM="fremen"
# [ -z $QUOTA_POLICY ] && QUOTA_POLICY="Qatreides"
[ -z $EXPORT_POLICY ] && EXPORT_POLICY="fremen-projects"
[ -z $PATH_PROJECT ] && PATH_PROJECT="/project"
[ -z $UID_PROJECT ] && UID_PROJECT="1010"
[ -z $GID_PROJECT ] && GID_PROJECT="1010"

API_URL="${API_HOST}/api"

# curl command prefix
CURL="curl -k -#"

# Print usage message and document.
function usage() {
    echo "Usage: $1 <new|get|qot|del> <projectID|username> [<sizeGb>] [<usergroup>]" 1>&2
    cat << EOF >&2

This script is a demo implementation of managing project flexvol/qtree
using the ONTAP management APIs. It requires "curl" and "jq".

API documentation: https://library.netapp.com/ecmdocs/ECMLP2856304/html/index.html 

Environment variables:
            API_HOST: URL of the API host.
            API_USER: username for accessing the API server.
            API_PASS: password for accessing the API server (prompt for password if not set).
                 SVM: vserver name.
       EXPORT_POLICY: NAS export policy name
        PATH_PROJECT: NAS export path (not used if project is made as qtree).
         UID_PROJECT: numerical uid of the user "project"
         GID_PROJECT: numerical gid of the group "project_g"

Operations:
    new: creates new space for a project or a user home.
    get: retrieves information of the space.
    qot: configures quota of the project/user home space.
    del: deletes space of a project or a user home.
EOF
}

# Check if given argument is a project name
function isProjectName() {
    [[ $1 =~ ^[0-9]{7}\.[0-9]{2}$ ]]
}

# Convert projectID to volume name using a convention
function projectVolName() {
    echo "project_${1}" | sed 's/\./_/g'
}

# Convert projectID to NAS path using a convention
function projectNasPath() {
    echo "${PATH_PROJECT}/${1}"
}

# Get the API href to the named object. 
function getHrefByQuery() {
    query=$1
    api_ns=$2
    ${CURL} -X GET -u ${API_USER}:${API_PASS} "${API_URL}/${api_ns}?${query}" | \
    jq '.records[] | ._links.self.href' | \
    sed 's/"//g'
}

# Get Object attributes by UUID in a generic way.
function getObjectByUUID() {

    uuid=$1
    api_ns=$2

    filter=".|.$(echo ${@:3} | sed 's/ /,./g')"

    ${CURL} -X GET -u ${API_USER}:${API_PASS} "${API_URL}/${api_ns}/${uuid}" | \
    jq ${filter}
}

# Get Object attributes by Href in a generic way.
function getObjectByHref() {
    href=$1
    filter=".|.$(echo ${@:2} | sed 's/ /,./g')"

    ${CURL} -X GET -u ${API_USER}:${API_PASS} "${API_HOST}/${href}" | \
    jq ${filter}
}

# Get Object attributes by name in a generic way.
function getObjectByName() {
    name=$1
    api_ns=$2

    href=$(getHrefByQuery "name=$name" $api_ns)

    [ "" == "$href" ] && echo "object not found: $name" >&2 && return 1
   
    getObjectByHref $href ${@:3}
}

# Creates a new qtree with given qtree path
function newQtree() {
    name=$1
    volname=$2

    # first get volume uuid
    voluuid=$(getObjectByName $volname '/storage/volumes' 'uuid' 2>/dev/null)
    [ "$voluuid" == "" ] && echo "volume doesn't exist: $volname" >&2 && return 1

    # check if tree already exists
    href=$( getHrefByQuery "name=$name" '/storage/qtrees' 2>/dev/null )
    [ "$href" != "" ] &&
        echo "qtree already exists: $name" >&2 && return 1

    # create new qtree
    out=$( ${CURL} -X POST -u ${API_USER}:${API_PASS} \
            -H 'content-type: application/json' \
            -d $(dataQtree $name $volname) \
            ${API_URL}/storage/qtrees )

    [ "$(echo $out | jq '.error' )" != "null" ] &&
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
    out=$( ${CURL} -X POST -u ${API_USER}:${API_PASS} \
            -H 'content-type: application/json' \
            -d $(dataProjectVolume $name $quota $aggr $path) \
            ${API_URL}/storage/volumes )

    [ "$(echo $out | jq '.error' )" != "null" ] &&
        echo "fail to create volume: $name" >&2 &&
        echo $out | jq && return 1

    echo $out | jq
}

# Set volume size
function resizeVolume() {
    name=$1
    quota=$( echo "$2 * 1024 * 1024 * 1024" | bc )

    # check if volume exists.
    href=$(getHrefByQuery "name=$name" '/storage/volumes' 2>/dev/null)
    [ "$href" == "" ] && echo "volume does not exists: $name" >&2 && return 1

    # resizing volume 
    out=$( ${CURL} -X PATCH -u ${API_USER}:${API_PASS} \
            -H 'content-type: application/json' \
            -d $(dataResizeVolume $name $quota) \
            ${API_HOST}/${href} )

    [ "$(echo $out | jq '.error' )" != "null" ] &&
        echo "fail to resize volume: $name" >&2 &&
        echo $out | jq && return 1

    echo $out | jq
}

# Get quota rule
function getQuotaRule() {
    name=$1

    href=$(getHrefByQuery "qtree.name=$name" '/storage/quota/rules')
    [ "" == "$href" ] && echo "quota rule not found: $name" && return 1

    getObjectByHref $href ${@:2}
}

# Create quota rule
function newQuotaRule() {
    name=$1   # name is the name in the context of the rule type
    volname=$2
    quota=$( echo "$3 * 1024 * 1024 * 1024" | bc )

    # make sure volume is presented
    href=$(getHrefByQuery "name=$volname" '/storage/volumes' 2>/dev/null)
    [ "" == "$href" ] &&
        echo "volume doesn't exists: $volname" >&2 &&
        return 1

    # make sure the quota rule name doesn't exist in the volume
    href=$(getHrefByQuery "qtree.name=$name&volume.name=$volname" '/storage/quota/rules' 2>/dev/null)
    [ "" != "$href" ] &&
        echo "quota rule already exists: $name" >&2 &&
        return 1

    # NOTE: it seems that switching off quota on volume is necessary in
    #       order to create an explicit quota rule.
    # NOTE: it is no longer needed to turn off/on the volume quota if there is already a
    #       default rule applied.  !!Manually create the default rule is required!!
    # switchVolumeQuota $volname off || return 1

    # create quota rule ...
    out=$( ${CURL} -X POST -u ${API_USER}:${API_PASS} \
            -H 'content-type: application/json' \
            -d $(dataQuotaRule $name $volname $quota) \
            ${API_URL}/storage/quota/rules )

    [ "$(echo $out | jq '.error' )" != "null" ] &&
        echo "fail to create quota rule: $name" >&2 &&
        echo $out | jq && return 1

    # turn quota off and on to refresh the quota
    # NOTE: it is no longer needed to turn off/on the volume quota if there is already a
    #       default rule applied.  !!Manually create the default rule is required!!
    # switchVolumeQuota $volname off &&
    # switchVolumeQuota $volname on ||
    # (echo "unable to refresh quota on volume: $volname" >&2 && return 1) 
}

# update the quota rule to resize quota of a qtree 
function resizeQuota() {
    name=$1   # name is the name in the context of the rule type
    volname=$2   # volume name is the name in the context of the rule type
    quotaGb=$3
    quota=$( echo "$quotaGb * 1024 * 1024 * 1024" | bc )

    # make sure the quota rule is presented
    href=$(getHrefByQuery "qtree.name=$name&volume.name=$volname" '/storage/quota/rules' 2>/dev/null)
    [ "" == "$href" ] &&
        echo "specific quota rule doesn't exists: $name, volume $volname" >&2 &&
            echo "creating new quota rule for $name, volume $volname" >/dev/tty &&
            newQuotaRule $name $volname $quotaGb && return $?

    # set quota rule ...
    out=$( ${CURL} -X PATCH -u ${API_USER}:${API_PASS} \
            -H 'content-type: application/json' \
            -d $(dataResizeQuota $quota) \
            ${API_HOST}/${href} )

    [ "$(echo $out | jq '.error' )" != "null" ] &&
        echo "fail to set quota rule: $name" >&2 &&
        echo $out | jq && return 1

    # turn quota off and on to refresh the quota
    # NOTE: it is no longer needed to turn off/on the volume quota if there is already a
    #       default rule applied.  !!Manually create the default rule is required!!
    # switchVolumeQuota $volname off &&
    # switchVolumeQuota $volname on ||
    # (echo "unable to refresh quota on volume: $volname" >&2 && return 1)
}

# switch on/off quota for a volume, and wait until it is applied.
function switchVolumeQuota() {
   
    volname=$1
    state=$2

    # make sure the volume is presented
    href=$(getHrefByQuery "name=$volname" '/storage/volumes' 2>/dev/null)
    [ "" == "$href" ] &&
        echo "volume doesn't exists: $volname" >&2 &&
        return 1

    data=""
    case $state in
    on)
        data='{"quota":{"enabled":true}}'
        ;;

    off)
        data='{"quota":{"enabled":false}}'
        ;;
    *)
        echo "unknown state" >&2
        return 1
        ;;
    esac

    out=$( ${CURL} -X PATCH -u ${API_USER}:${API_PASS} \
            -H 'content-type: application/json' \
            -d $(echo $data) \
            ${API_HOST}/${href} )

    [ "$(echo $out | jq '.error' )" == "null" ] ||
        (echo "fail to switch quota state on volume: $volname" >&2 && return 1)

    # wait for the job to complete
    jid=$(echo $out | jq '.job.uuid' | sed 's/"//g')
    while [[ ! $(getJobState $jid) =~ ^(success|failure)$ ]]; do
        sleep 1
    done

    # check final state
    case "$(getJobState $jid)" in
    success)
        return 0
        ;;
    failure)
        getJobMessage $jid | grep "Quotas are already $state" >/dev/null && return 0 || return 1
        ;;
    *)
        return 1
        ;;
    esac
}

# Get job status
function getJobState() {
    id=$1
    ${CURL} -X GET -u ${API_USER}:${API_PASS} \
        "${API_URL}/cluster/jobs/${id}" | jq '.state' | sed 's/"//g'
}

# Get job messages
function getJobMessage() {
    id=$1
    ${CURL} -X GET -u ${API_USER}:${API_PASS} \
        "${API_URL}/cluster/jobs/${id}" | jq '.message' | sed 's/"//g'
}

# Compose POST data for creating quota rule
function dataQuotaRule() {
    name=$1
    volname=$2
    diskLimit=$3

    jq --arg name "$name" \
       --arg volname "$volname" \
       --arg svm "$SVM" \
       --arg limit "$diskLimit" \
       -c -M \
       '.|.qtree.name=$name|.volume.name=$volname|.svm.name=$svm|.space.hard_limit=($limit|tonumber)' << EOF

{
  "svm": {},
  "volume": {},
  "type": "tree",
  "qtree": {},
  "space": {}
}

EOF

}

# Compose POST data for resizing quota on a qtree
function dataResizeQuota() {
    diskLimit=$1
    jq --arg limit "$diskLimit" \
       -c -M \
       '.|.space.hard_limit=($limit|tonumber)' << EOF

{
  "space": {}
}

EOF

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
  },
  "snapshot_policy": {
    "name": "none"
  }
}

EOF

}

## main program
[ $# -lt 2 ] && usage $0 && exit 1
ops=$1

## prompt to ask for API_PASS if not set
[ -z $API_PASS ] &&
    echo -n "Password for API user ($API_USER): " &&  read -s API_PASS && echo

case $ops in
new)

    [ $# -lt 3 ] && usage $0 && echo "missing size" >&2 && exit 1
    sizeGb=$3

    if isProjectName $2; then #this is for project
        projectID=$2

        ### For creating project as qtree.
        echo "Creating qtree for project $projectID, volume project ... " &&
        newQtree $projectID project &&
            newQuotaRule $projectID project $sizeGb || exit 1

        ### For creating project as volume.
        # volname=$(projectVolName $projectID)
        # nasPath=$(projectNasPath $projectID)
        # echo "Creating volume of project $projectID ..." &&
        # newVolume $volname $sizeGb $nasPath || exit 1
    else #this is for user home
        [ $# -lt 4 ] && usage $0 && echo "missing user group" >&2 && exit 1
        name=$2
        # TODO: determine volume name from user's primary group
        volname=$4
        echo "Creating qtree for user $name, group $volname ..." &&
        newQtree $name $volname || exit 1

        # set new quota rule for home qtree if non-default ($sizeGb != 0 ).
        [ $sizeGb -ne 0 ] && newQuotaRule $name $volname $sizeGb || exit 1
    fi

    ;;
get)
    if isProjectName $2; then #this is for project
        projectID=$2

        ### For project as qtree.
        echo "Getting qtree of project $projectID ..." &&
        getObjectByName $name "/storage/qtrees" && 
        echo "Getting quota rule for project $projectID ..." &&
        getQuotaRule $name || exit 1

        ### For project as volume.
        # echo "Getting volume of project $projectID ..." &&
        # getObjectByName $(projectVolName $projectID) "/storage/volumes" || exit 1
    else #this is for user home
        name=$2
        echo "Getting qtree of user $name ..." &&
        getObjectByName $name "/storage/qtrees" && 
        echo "Getting quota rule for user $name ..." &&
        getQuotaRule $name || exit 1
    fi
    ;;
del)
    ;;
qot)
    [ $# -lt 3 ] && usage $0 && exit 1
    sizeGb=$3

    if isProjectName $2; then #this is for project
        projectID=$2

        ### For project as qtree.
        echo "Setting quota for project $projectID ..." &&
        resizeQuota $projectID project $sizeGb || exit 1     

        ### For project as volume.
        # volName=$(projectVolName $projectID)

        # echo "Resizing volume for project $projectID ..." &&
        # resizeVolume $volName $sizeGb || exit 1
    else #this is for user home
        [ $# -lt 4 ] && usage $0 && echo "missing user group" >&2 && exit 1
        name=$2
        # TODO: determine volume name from user's primary group
        volname=$4

        echo "Setting quota for user $name ..." &&
        resizeQuota $name $volname $sizeGb || exit 1
    fi
    ;;
*)
    echo "unknown operation: $ops" >&2 && exit 1
    ;;
esac

exit 0
