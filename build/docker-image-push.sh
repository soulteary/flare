#!/bin/bash
VERSION=$(git describe --tags --abbrev=0)
COMMIT=$(git rev-parse --short HEAD)

echo "最近版本；$VERSION / $COMMIT"

DOCKERHUB_REPO="soulteary/flare"

docker images | grep "$DOCKERHUB_REPO"

docker push "$DOCKERHUB_REPO:$VERSION-amd64"
docker push "$DOCKERHUB_REPO:$VERSION-arm32v7"
docker push "$DOCKERHUB_REPO:$VERSION-arm64v8"

docker manifest create "$DOCKERHUB_REPO:$VERSION"  \
                       "$DOCKERHUB_REPO:$VERSION-amd64" \
                       "$DOCKERHUB_REPO:$VERSION-arm32v7" \
                       "$DOCKERHUB_REPO:$VERSION-arm64v8" --amend
docker manifest push "$DOCKERHUB_REPO:$VERSION"
