IMAGE_SHA=`docker inspect --format='{{index .RepoDigests 0}}' quay.io/jpacker/clustercurator-job:0.6.3`
IMAGE_SHA=${IMAGE_SHA/quay.io\/jpacker\/clustercurator-job@}
oc process -f deploy/controller/template-deployment.yaml -p IMAGE_SHA=${IMAGE_SHA} -p REPO_URL=${REPO_URL} > deploy/controller/deployment.yaml
