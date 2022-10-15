DOCKER_REPO_USER = ${USER}
IMAGE_NAME_PREFIX = voda-
IMAGE_TAG = 0.2.0
IMAGE_TAG_LATEST = latest
RELEASE_NAME = voda-scheduler

# Voda scheduler deploys GPU scheduler for each specified GPU type
# GPU type should match node label: vodascheduler/accelerator=<GPU_TYPE_OF_THE_NODE>
## TODO: have the following set
GPU_TYPES = nvidia-gtx-1080ti nvidia-tesla-v100

gen-scheduler:
	cd helm/voda-scheduler && bash gen-scheduler-yaml.sh ${GPU_TYPES}

deploy-all: gen-scheduler
	helm install ${RELEASE_NAME} ./helm/voda-scheduler

delete-all:
	helm delete ${RELEASE_NAME}

docker-build-training-service:
	docker build -f docker/training-service/Dockerfile . -t ${DOCKER_REPO_USER}/${IMAGE_NAME_PREFIX}training-service:${IMAGE_TAG}

docker-build-scheduler:
	docker build -f docker/vodascheduler/Dockerfile . -t ${DOCKER_REPO_USER}/${IMAGE_NAME_PREFIX}scheduler:${IMAGE_TAG}

docker-build-resource-allocator:
	docker build -f docker/resource-allocator/Dockerfile . -t ${DOCKER_REPO_USER}/${IMAGE_NAME_PREFIX}resource-allocator:${IMAGE_TAG}

docker-build-metrics-collector:
	docker build -f docker/metrics-collector/Dockerfile . -t ${DOCKER_REPO_USER}/${IMAGE_NAME_PREFIX}metrics-collector:${IMAGE_TAG}

docker-build-all: docker-build-training-service docker-build-scheduler docker-build-resource-allocator docker-build-metrics-collector

docker-push-training-service:
	docker push ${DOCKER_REPO_USER}/${IMAGE_NAME_PREFIX}training-service:${IMAGE_TAG}

docker-push-scheduler:
	docker push ${DOCKER_REPO_USER}/${IMAGE_NAME_PREFIX}scheduler:${IMAGE_TAG}

docker-push-resource-allocator:
	docker push ${DOCKER_REPO_USER}/${IMAGE_NAME_PREFIX}resource-allocator:${IMAGE_TAG}

docker-push-metrics-collector:
	docker push ${DOCKER_REPO_USER}/${IMAGE_NAME_PREFIX}metrics-collector:${IMAGE_TAG}

docker-push-all: docker-push-training-service docker-push-scheduler docker-push-resource-allocator docker-push-metrics-collector

docker-build-and-push-all: docker-build-all docker-push-all

