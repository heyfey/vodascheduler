#!/bin/bash

# Sending GET request to scheduler
# $1 = API endpoint
# $2 = Scheduler name, curl all deployed scheduler if not specified

port=55588
namespace=voda-scheduler

if [ -n "$1" ]; then
    endpoint="$1"
    echo "Endpoint: $1"
    echo "---"

    if [ -n "$2" ]; then
        svcs=scheduler-$2-svc
    else
        svcs=$(kubectl get svc -n ${namespace} --no-headers -o custom-columns=":metadata.name" | grep '^scheduler.*svc$')
    fi

    for svc in ${svcs}
    do 
        echo "Service: ${svc}"
        ip=$(kubectl get services/${svc} -n ${namespace} -ojsonpath='{.spec.clusterIP}')
        url=${ip}:${port}${endpoint}
        cmd="curl -X GET ${url}"
        # echo "Cmd: ${cmd}"
        echo "---"
        ${cmd}
        echo -e "\n"
    done
else
    echo "Missing endpoint in the first argument. E.g. /training , /metrics"
fi