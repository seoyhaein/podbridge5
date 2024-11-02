#!/usr/bin/env bash

podman commit testContainer testcontainerimage

podman run --rm -it testcontainerimage /bin/bash
