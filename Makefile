DOCKER_REPO_USER = ${USER}
IMAGE_NAME_PREFIX = voda-


docker-build-training-service:
	docker build -f docker/training-service/Dockerfile . -t ${DOCKER_REPO_USER}/${IMAGE_NAME_PREFIX}training-service

docker-build-scheduler:
	docker build -f docker/vodascheduler/Dockerfile . -t ${DOCKER_REPO_USER}/${IMAGE_NAME_PREFIX}scheduler

docker-build-resource-allocator:
	docker build -f docker/resource-allocator/Dockerfile . -t ${DOCKER_REPO_USER}/${IMAGE_NAME_PREFIX}resource-allocator

docker-build-metrics-collector:
	docker build -f docker/metrics-collector/Dockerfile . -t ${DOCKER_REPO_USER}/${IMAGE_NAME_PREFIX}metrics-collector

docker-build-all: docker-build-training-service docker-build-scheduler docker-build-resource-allocator docker-build-metrics-collector

docker-push-training-service:
	docker push ${DOCKER_REPO_USER}/${IMAGE_NAME_PREFIX}training-service

docker-push-scheduler:
	docker push ${DOCKER_REPO_USER}/${IMAGE_NAME_PREFIX}scheduler

docker-push-resource-allocator:
	docker push ${DOCKER_REPO_USER}/${IMAGE_NAME_PREFIX}resource-allocator

docker-push-metrics-collector:
	docker push ${DOCKER_REPO_USER}/${IMAGE_NAME_PREFIX}metrics-collector

docker-push-all: docker-push-training-service docker-push-scheduler docker-push-resource-allocator docker-push-metrics-collector

docker-build-and-push-all: docker-build-all docker-push-all

