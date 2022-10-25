#!/bin/bash

# Sending GET request to training service
# $1 = API endpoint

port=55587
namespace=voda-scheduler
svc=voda-scheduler-svc

if [ -n "$1" ]; then
    endpoint="$1"
    echo "Endpoint: $1"
    echo "---"
    echo "Service: ${svc}"
    ip=$(kubectl get services/${svc} -n ${namespace} -ojsonpath='{.spec.clusterIP}')
    url=${ip}:${port}${endpoint}
    cmd="curl -X GET ${url}"
    # echo "Cmd: ${cmd}"
    echo "---"
    ${cmd}
    echo -e "\n"
else
    echo "Missing endpoint in the first argument. E.g. /metrics"
fi