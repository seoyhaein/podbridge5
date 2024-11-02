#!/bin/sh

if ! command -v bash >/dev/null 2>&1; then
    echo "Bash is not installed. Installing bash..."
    # alpine 만 테스트 했음. 필요한 utils.
    if command -v apk >/dev/null 2>&1; then
        apk update && apk add --no-cache bash coreutils util-linux nano procps
    elif command -v apt >/dev/null 2>&1; then
        apt update && apt install -y bash
    elif command -v yum >/dev/null 2>&1; then
        yum update -y && yum install -y bash
    else
        echo "Unknown package manager. Please install bash manually."
        exit 1
    fi
fi