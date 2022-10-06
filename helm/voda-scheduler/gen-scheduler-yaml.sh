#!/bin/bash

rm templates/scheduler/*.yaml
mkdir -p templates/scheduler/

while [ $# -gt 0 ]
do
  gpuType=$1
  target=templates/scheduler/scheduler-$gpuType.yaml
  echo $gpuType
  echo genarating: $PWD/$target
  sed "s/\$\$gpuTypeToBeOverride/$gpuType/g" scheduler.yaml.base > $target 
  shift
done