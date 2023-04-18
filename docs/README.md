# Debugging Cluster-Curator-Controller

After you make your changes you actually need to build your own image because when the curator code creates a Job CR, the Job will load its own cluster-curator-controller image to run the curator binary. Make sure docker is started.

1. export REPO_URL=<your_quay_url> ie. quay.io/fxiang1
2. export VERSION=<image_tag>
3. make build-curator
4. docker push <REPO_URL>/cluster-curator-controller:<VERSION>

## Debugging locally in VSCode

If you are debugging locally then you need to do the following:
1. oc login to the cluster
2. Stop the curator running on the cluster by first scaling down the `multicluster-engine-operator` pods and then the `cluster-curator-controller` pods
3. Add the IMAGE_URI env var pointing to the image you just pushed in the VSCode launch.json. If you don’t do this then it will just load the latest image which will not contain your changes

Example launch.json:
```json
{
 "version": "0.2.0",
 "configurations": [
   {
     "name": "Launch",
     "type": "go",
     "request": "launch",
     "mode": "debug",
     "program": "${workspaceRoot}/cmd/manager",              // this is where the cluster curator manager main go is
     "env": {
       "WATCH_NAMESPACE": "",
       "IMAGE_URI": "quay.io/fxiang1/cluster-curator-controller@sha256:ce527566269f4bffad08ae8eb8533c9f829406d8bfc09299f4a84fe5492666b5"
     },                         // set up additional env var here
     "args": [],
     "showLog": true
   }
 ]
}
```

When using breakpoints in VSCode you can only debug up to https://github.com/stolostron/cluster-curator-controller/blob/main/pkg/controller/launcher/job.go. For the code after that point, you still need to do scaffolding ie. use log or printf statements to debug because that code is called by the Job CRs which we can’t step through using VSCode.
