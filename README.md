# kubernetes-example-controller

An example Kubernetes controller for using a custom resource type.

Demo for a [Kubernetes Atlanta Meetup](https://www.meetup.com/Kubernetes-Atlanta-Meetup/)

Written in Go using only the standard library.

## What is this?

`kubernetes-example-controller` is an example controller that demonstrates
using a [custom resource definition](https://kubernetes.io/docs/concepts/api-extension/custom-resources/). It is 
just a demonstration.

The file [heap.yaml](./heap.yaml) defines a custom resource called a "heap". 
(Why is it called a heap? I am terrible at naming things, that's why.) A heap is
a simple way to declare a [deployment](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/) and optionally create a [service](https://kubernetes.io/docs/concepts/services-networking/service/) and an [ingress](https://kubernetes.io/docs/concepts/services-networking/ingress/). 

Note: this code is intended to be straight forward and avoid being "clever." It
is not meant to be exemplary Go code.  It is intended to demonstrate working
with Kubernetes resources with little abstraction.

### Heap

A Heap has the following fields:

See also: [Kubernetes metadata fields](https://kubernetes.io/docs/concepts/overview/working-with-objects/kubernetes-objects/#required-fields)

```yaml
apiVersion: akins.org/v1alpha1
kind: Heap
metadata:
    name: <name-of-resource>
    namespace: <namespace>
    labels: <key/value pairs>
    annotations: <key/value pairs>
spec:
    image: <image-to-run>
    command: array of strings to run (optional)
    replicas: defaults to 1 - number of pod replicas to run
    port: port to expose. used for the service and ingress
    host: host to use for ingress
```

The `name` is used as the name for the deployment, service, and ingress that
the controller may create for this heap.

If a `port` is specified, then a service is also created that uses this port for both the [service](https://kubernetes.io/docs/concepts/services-networking/service/) and target port. If host is also set, an [ingress](https://kubernetes.io/docs/concepts/services-networking/ingress/) is created
using this host.

There are a few [examples](./examples) included in the repository.

## How to use it

To run this an a demo on you local workstation, you need a working local Kubernetes cluster.  [Docker for Mac](https://docs.docker.com/docker-for-mac/kubernetes/) or [Minikube](https://github.com/kubernetes/minikube) are good ways to do this.

You also need a working [Go development environment](https://golang.org/doc/install).

Clone this repository into your GOPATH:

```
mkdir -p $(go env GOPATH)/src/github.com/bakins
cd $(go env GOPATH)/src/github.com/bakins
git clone https://github.com/bakins/kubernetes-example-controller.git
cd kubernetes-example-controller
```


### Custom Resource Definition

Ensure your local Kubernetes environment is working:

```shell
$ kubectl get nodes
NAME                 STATUS    ROLES     AGE       VERSION
docker-for-desktop   Ready     master    25d       v1.9.2
```

(Note: your output may vary). If this doesn't work, consult the documentation for
Docker or Minikube.

Create the custom resource:

```shell
$ kubectl apply -f ./heap.yaml
customresourcedefinition "heaps.akins.org" configured
```

You should now be able to list heaps:

```shell
$ kubectl get heaps
No resources found.
```

### Controller

In another terminal, run `kubectl proxy`. This will run a proxy to the Kubernetes
API. For simplicity, the example controller does not handle authentication and
relies on the proxy to handle it.

In a third terminal run:

```
cd $(go env GOPATH)/src/github.com/bakins/kubernetes-example-controller
go run *.go
```

This will run the controller which will access the Kubernetes API using the local proxy.

The controller itself is fairly simple.  It lists all heaps every 10 seconds and then acts on them. A real controller would watch the API and act as changes are made.  Once again, this was done as a demonstration and for simplicity. There is a more featureful [sample controller'(https://github.com/kubernetes/sample-controller) if you'd like to explore this more.

### Creating a Heap

In the first terminal in the repository directory, run

```shell
$ kubectl apply -f ./examples/no-service.yaml
heap "no-service" created
```

You can see this heap by running:

```shell
$ kubectl get heaps
NAME         AGE
no-service   59s

$ kubectl descibe heap no-service
Name:         no-service
Namespace:    default
...
Events:
  Type  Reason             Age   From  Message
  ----  ------             ----  ----  -------
        deploymentCreated  1m          created deployment
```

Notice the events associated with the heap.

This will create a heap that has no port defined, so it will just create a deployment. To see this run:

```shell
$ kubectl get deploy no-service
NAME         DESIRED   CURRENT   UP-TO-DATE   AVAILABLE   AGE
no-service   1         1         1            1           2m

$ kubectl get service no-service
Error from server (NotFound): services "no-service" not found
```

Now, let's delete this heap and see that the resources are removed automatically
using [Kubernetes garbage collection](https://kubernetes.io/docs/concepts/workloads/controllers/garbage-collection/):

```shell
$ kubectl delete heaps no-service
heap "no-service" deleted

$ sleep 10

$ kubectl get deploy no-service
Error from server (NotFound): deployments.extensions "no-service" not found
```

Finally, let's create a heap that includes a service and an ingress:

```shell
$ kubectl apply -f ./examples/with-ingress.yaml
heap "with-ingress" created

$ kubectl describe heap with-ingress
Name:         with-ingress
Namespace:    default
...
Events:
  Type  Reason             Age   From  Message
  ----  ------             ----  ----  -------
        deploymentCreated  13s         created deployment
        serviceCreated     13s         created service
        ingressCreated     13s         created ingress

$ kubectl get deploy with-ingress
NAME           DESIRED   CURRENT   UP-TO-DATE   AVAILABLE   AGE
with-ingress   1         1         1            1           1m

$ kubectl get service with-ingress
NAME           TYPE        CLUSTER-IP      EXTERNAL-IP   PORT(S)   AGE
with-ingress   ClusterIP   10.109.117.12   <none>        80/TCP    1m

$ kubectl get ingress with-ingress
NAME           HOSTS                ADDRESS   PORTS     AGE
with-ingress   somehost.akins.org             80        2m
```


Now we can remove it and see that the related resources are deleted:

```shell
$ kubectl delete heap with-ingress
heap "with-ingress" deleted

$ kubectl get deploy with-ingress
Error from server (NotFound): deployments.extensions "with-ingress" not found

```

To remove the custom resource run:

```shell
$ kubectl delete crd heaps.akins.org
customresourcedefinition "heaps.akins.org" deleted
```


## LICENSE

See [LICENSE](./LICENSE)


