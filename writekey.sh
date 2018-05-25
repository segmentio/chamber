#!/usr/bin/env bash

if [ "$#" -ne 4 ]; then
    echo "Invalid ammount of parameters"
    echo "Example: ./writekey.sh CLUSTER SERVICE KEY_NAME KEY_VALUE"
    exit 1
fi

which okta >> /dev/null ||  { echo "You must install 'okta'. Please follow the README at https://github.com/sagansystems/gladly-okta-cli"; exit 1; }

okta login

docker run -it -v ~/.aws:/home/gladly/.aws -e AWS_PROFILE -e AWS_SDK_LOAD_CONFIG=1 -e CHAMBER_KMS_KEY_ALIAS=aws/ssm -e AWS_REGION=us-east-1 -e CLUSTER=$1 sagan/chamber:latest write $2/secrets $3 $4
docker run -it -v ~/.aws:/home/gladly/.aws -e AWS_PROFILE -e AWS_SDK_LOAD_CONFIG=1 -e CHAMBER_KMS_KEY_ALIAS=aws/ssm -e AWS_REGION=us-east-2 -e CLUSTER=$1 sagan/chamber:latest write $2/secrets $3 $4
docker run -it -v ~/.aws:/home/gladly/.aws -e AWS_PROFILE -e AWS_SDK_LOAD_CONFIG=1 -e CHAMBER_KMS_KEY_ALIAS=aws/ssm -e AWS_REGION=us-west-1 -e CLUSTER=$1 sagan/chamber:latest write $2/secrets $3 $4
docker run -it -v ~/.aws:/home/gladly/.aws -e AWS_PROFILE -e AWS_SDK_LOAD_CONFIG=1 -e CHAMBER_KMS_KEY_ALIAS=aws/ssm -e AWS_REGION=us-west-2 -e CLUSTER=$1 sagan/chamber:latest write $2/secrets $3 $4