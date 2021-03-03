IMAGE_SHA=`docker inspect --format='{{index .RepoDigests 0}}' ${REPO_URL}/cluster-curator-controller:${VERSION}`

IMAGE_SHA=${IMAGE_SHA#*@}  #Extract the SHA256
echo "Container digest: \"${IMAGE_SHA}\" in repository: ${REPO_URL}"
oc process -f deploy/controller/template-deployment.yaml -p IMAGE_SHA=${IMAGE_SHA} -p REPO_URL=${REPO_URL} -o yaml --raw=true > deploy/controller/deployment.yaml
