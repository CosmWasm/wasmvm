# Versioned by a simple counter that is not bound to a specific CosmWasm version
# See builders/README.md
BUILDERS_PREFIX := cosmwasm/go-ext-builder:0009

.PHONY: docker-image-centos7
docker-image-centos7:
ifdef ARM64
	docker build --platform linux/arm64 . -t $(BUILDERS_PREFIX)-centos7 -f ./Dockerfile.centos7 --build-arg arch=aarch64
else
	docker build . -t $(BUILDERS_PREFIX)-centos7 -f ./Dockerfile.centos7
endif

.PHONY: docker-image-cross
docker-image-cross:
	docker build . -t $(BUILDERS_PREFIX)-cross -f ./Dockerfile.cross

.PHONY: docker-image-alpine
docker-image-alpine:
	docker build . -t $(BUILDERS_PREFIX)-alpine -f ./Dockerfile.alpine

.PHONY: docker-images
docker-images: docker-image-centos7 docker-image-cross docker-image-alpine

.PHONY: docker-publish
docker-publish: docker-images
	docker push $(BUILDERS_PREFIX)-cross
	docker push $(BUILDERS_PREFIX)-centos7
	docker push $(BUILDERS_PREFIX)-alpine
