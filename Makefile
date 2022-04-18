OUT := tfo
PKG := github.com/isaaguilar/terraform-operator-cli
VERSION ?= $(shell git ls-remote .|grep $$(git rev-parse HEAD).*tags|head -n1|sed "s/^.*\///")
ifeq ($(VERSION),)
VERSION := v0.0.0
endif
OS := $(shell uname -s | tr A-Z a-z)

all: clean run

arch-envs:
DARWIN_AMD = "darwin-amd64"
LINUX_AMD = "linux-amd64"
LINUX_ARM = "linux-arm64"


check-version:
	echo ${VERSION}

build:
	env GOOS=${DARWIN_AMD} GOARCH=amd64 go build -i -v -o ${OUT}-${DARWIN_AMD} -ldflags="-X main.version=${VERSION}" ${PKG}
	env GOOS=${LINUX_AMD} GOARCH=amd64 go build -i -v -o ${OUT}-${LINUX_AMD} -ldflags="-X main.version=${VERSION}" ${PKG}
	env GOOS=${LINUX_ARM} GOARCH=amd64 go build -i -v -o ${OUT}-${LINUX_ARM} -ldflags="-X main.version=${VERSION}" ${PKG}



# update-installer:
# 	mkdir -p gen && echo "defaultVersion: ${VERSION}" > gen/values.yaml
# 	gt hack/install-opsbox.tpl.sh -f gen/values.yaml > install-opsbox.sh
# 	curl -H "X-JFrog-Art-Api:${ARTIFACTORY_TOKEN}" -T ./install-opsbox.sh "https://artifactory.bf-aws.illumina.com/artifactory/archive-eibu-internal/opsbox/install-opsbox.sh"

run: build
	[[ ${OS} == "${LINUX_AMD}"* ]] && ./${OUT}-${LINUX_AMD} version ||:
	[[ ${OS} == "${LINUX_ARM}"* ]] && ./${OUT}-${LINUX_ARM} version ||:
	[[ ${OS} == "${DARWIN_AMD}"* ]] && ./${OUT}-${DARWIN_AMD} version ||:
	mv ./${OUT}-${LINUX_AMD} ./${OUT} && tar czf ${OUT}-${VERSION}-${LINUX_AMD}.tgz ${OUT}
	mv ./${OUT}-${LINUX_ARM} ./${OUT} && tar czf ${OUT}-${VERSION}-${LINUX_ARM}.tgz ${OUT}
	mv ./${OUT}-${DARWIN_AMD} ./${OUT} && tar czf ${OUT}-${VERSION}-${DARWIN_AMD}.tgz ${OUT}


clean:
	-@rm ${OUT} ${OUT}-v*

.PHONY: run build