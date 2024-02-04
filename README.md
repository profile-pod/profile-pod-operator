# profile-pod-operator
A Kubernetes operator for profiling applications running inside pods with low-overhead.

## Description
The profile-pod-operator visualizing CPU time spent in the application functions using [FlameGraph](https://www.brendangregg.com/flamegraphs.html). The operator's goal is to help you pin down where your application spend too much time with low-overhead and without any modification. By not require any modification or restart for existing applications and by using low-overhead profilers, this is great for recording flame graph data from an already running application in production environment that you don't want to interrupt. 

## Profile an application
To profile an application, a `PodFlame` custom resource needs to be created in the same namespace as the target application. Run the following command to create it:

```sh
cat << EOF | kubectl apply -f -
apiVersion: profilepod.io/v1alpha1
kind: PodFlame
metadata:
  name: my-app-flame
  namespace: my-app-namespace
spec:
  targetPod: my-app-54674f9647-jvm98 # The name of the pod you want to profile.
EOF
```
The following optional fields can be added to the PodFlame resource spec:

```yaml
    duration: 30s # The profiling duration in seconds (s/S) or minutins (m/M). default: 2m.
    containerName: myapp # Require when the pod contains more then one container. 
```
> Note: the `PodFlame` resource is immutable, if changes are required to a `PodFlame` resource, destroying the current resource and rebuilding that resource with required changes.


After PodFlame resource is created, an [agent pod](https://github.com/profile-pod/profile-pod-agent) will be created by the operator in the same node as the target pod who was specified in the PodFlame spec.
The agent pod is a high privileged pod, which detect the target application programming language and the target application process id, and runs a profiler suitable for the requested application. Once the Profile is done and flamegraph is generated for the application, it is placed in the `.status.flameGraph` of the corresponding PodFlame resource. Run the following command to get it: 

```sh
kubectl get pf my-app-flame -n my-app-namespace -o jsonpath='{.status.flameGraph}' | base64 -d | gunzip > myapp-flamegraph.html
```


> Note: the high privileged agent pod is created in the operator namespace, therefore, allow any unrestrictive policy in all profiled namespaces when using [Pod Security admission controller](https://kubernetes.io/docs/concepts/security/pod-security-admission/) (PSA) or similar enforcement tools should not be a concern. 
## Getting Started
You’ll need a Kubernetes cluster to run against. You can use [KIND](https://sigs.k8s.io/kind) to get a local cluster for testing, or run against a remote cluster.
**Note:** Your controller will automatically use the current context in your kubeconfig file (i.e. whatever cluster `kubectl cluster-info` shows).

### Running on the cluster
1. Install Instances of Custom Resources:

```sh
kubectl apply -f config/samples/
```

2. Build and push your image to the location specified by `IMG`:

```sh
make docker-build docker-push IMG=<some-registry>/profile-pod-operator:tag
```

3. Deploy the controller to the cluster with the image specified by `IMG`:

```sh
make deploy IMG=<some-registry>/profile-pod-operator:tag
```

### Uninstall CRDs
To delete the CRDs from the cluster:

```sh
make uninstall
```

### Undeploy controller
UnDeploy the controller from the cluster:

```sh
make undeploy
```

## Contributing
// TODO(user): Add detailed information on how you would like others to contribute to this project

### How it works
This project aims to follow the Kubernetes [Operator pattern](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/).

It uses [Controllers](https://kubernetes.io/docs/concepts/architecture/controller/),
which provide a reconcile function responsible for synchronizing resources until the desired state is reached on the cluster.

### Test It Out
1. Install the CRDs into the cluster:

```sh
make install
```

2. Run your controller (this will run in the foreground, so switch to a new terminal if you want to leave it running):

```sh
make run
```

**NOTE:** You can also run this in one step by running: `make install run`

### Modifying the API definitions
If you are editing the API definitions, generate the manifests such as CRs or CRDs using:

```sh
make manifests
```

**NOTE:** Run `make --help` for more information on all potential `make` targets

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)

## License

Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

