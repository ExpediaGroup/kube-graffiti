NAME   := hotelsdotcom/kube-graffiti
TAG    := $(shell git describe --tags)
IMG    := ${NAME}:${TAG}
LATEST := ${NAME}:latest

build:test
	@docker build -t "${IMG}" .
	@docker tag ${IMG} ${LATEST}
 
push:
	@docker push ${NAME}
 
login:
	@docker log -u ${DOCKER_USER} -p ${DOCKER_PASS}

chart:
	@helm package --app-version ${TAG} ./helm/kube-graffiti

test:
	@go test ./...
