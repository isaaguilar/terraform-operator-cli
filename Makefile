OUT := tfo
PKG := github.com/isaaguilar/terraform-operator-cli
VERSION ?= $(shell git ls-remote .|grep $$(git rev-parse HEAD).*tags|head -n1|sed "s/^.*\///")
ifeq ($(VERSION),)
VERSION := v0.0.0
endif
RELEASES := .rmgmt/releases/${VERSION}

all: clean run

check-version:
	echo ${VERSION}

build: check-version
	mkdir ${RELEASES}
	env GOOS=darwin GOARCH=amd64 go build -v -o ${RELEASES}/${OUT}-darwin-amd64 -ldflags="-X main.version=${VERSION}" ${PKG}
	env GOOS=linux GOARCH=amd64 go build -v -o ${RELEASES}/${OUT}-linux-amd64 -ldflags="-X main.version=${VERSION}" ${PKG}
	env GOOS=linux GOARCH=arm64 go build -v -o ${RELEASES}/${OUT}-linux-arm64 -ldflags="-X main.version=${VERSION}" ${PKG}
	mv ${RELEASES}/${OUT}-linux-amd64 ${RELEASES}/${OUT} && cd ${RELEASES} && tar czf ${OUT}-${VERSION}-linux-amd64.tgz ${OUT}
	mv ${RELEASES}/${OUT}-linux-arm64 ${RELEASES}/${OUT} && cd ${RELEASES} && tar czf ${OUT}-${VERSION}-linux-arm64.tgz ${OUT}
	mv ${RELEASES}/${OUT}-darwin-amd64 ${RELEASES}/${OUT} && cd ${RELEASES} && tar czf ${OUT}-${VERSION}-darwin-amd64.tgz ${OUT}

release: build
	/bin/bash hack/release.sh


# update-installer:
# 	mkdir -p gen && echo "defaultVersion: ${VERSION}" > gen/values.yaml
# 	gt hack/install-opsbox.tpl.sh -f gen/values.yaml > install-opsbox.sh
# 	curl -H "X-JFrog-Art-Api:${ARTIFACTORY_TOKEN}" -T ./install-opsbox.sh "https://artifactory.bf-aws.illumina.com/artifactory/archive-eibu-internal/opsbox/install-opsbox.sh"

run: build



clean:
	-@rm ${OUT} ${OUT}-v*

.PHONY: run build