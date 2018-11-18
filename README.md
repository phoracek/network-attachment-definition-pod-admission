# NetworkAttachmentDefinition Pod Admission

**WIP**

- **TODO:**
- overview
- multi-purpose, generic
- can be deployed multiple times
- only applies given json patch on a pod
- definition of this patch can access attributes of requested network using jinja2
- check out following example scenarios to get better idea

## Scenario 1: Schedule pod on a node with available physical interface

- **TODO:**
- first example, users can request access to a physical interface
- number of such interfaces is limited
- how do we make sure that pod is correctly scheduled?
- first, user must implement a daemon set (link) that would expose available connections as resources,
- read more about dp, but as an effect, in case a node has 4 nics connected to specific network, dp will expose available resources on node like this: example from yaml
- connection to this network is specified using following nad: example
- finally, there is nadpd deployed with following configuration: example
- thanks to that, for every network of type X, pod will get applied resource request on its first container
- so this pod definition: example
- will end up like this: example

## Scenario 2: Schedule pod on a node with pre-configured bridge

- **TODO:**
- second scenario is similar, except now we are not dealing with limited resource of physical nics, but availability of a bridge configured on a host
- so let's say, half of your nodes have preconfigured bridge with access to your very special network
- you need to make sure that pods requesting access to this network will be scheduled on given pods
- this time we will use node labeling and nodeSelectors
- node with this special bridge will have label X, labeling can be done manually or by a daemon provided by administrator
- nad to this bridge looks like this: example
- notice that there is a label Y, you will see it used later in configmap
- configuration looks like this: example
- so a pods that requests the network: example
- will end up like this: example
- and therefore scheduled on our node

## Deployment

- **TODO:**
- nad definition (multus)
- certificates
- building manifests
- configmap
- installing admission
- usage example

Create NetworkAttachmentDefinition CRD:

```shell
cat <<EOF | kubectl create -f -
apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  name: network-attachment-definitions.k8s.cni.cncf.io
spec:
  group: k8s.cni.cncf.io
  version: v1
  scope: Namespaced
  names:
    plural: network-attachment-definitions
    singular: network-attachment-definition
    kind: NetworkAttachmentDefinition
    shortNames:
    - net-attach-def
  validation:
    openAPIV3Schema:
      properties:
        spec:
          properties:
            config:
              type: string
EOF
```

Create example of a NetworkAttachmentDefinition:

```shell
cat <<EOF | kubectl create -f -
apiVersion: "k8s.cni.cncf.io/v1"
kind: NetworkAttachmentDefinition
metadata:
  name: macvlan-conf
spec:
  config: '{
      "cniVersion": "0.3.0",
      "type": "macvlan",
      "master": "eth0",
      "mode": "bridge",
      "ipam": {
        "type": "host-local",
        "subnet": "192.168.1.0/24",
        "rangeStart": "192.168.1.200",
        "rangeEnd": "192.168.1.216",
        "routes": [
          { "dst": "0.0.0.0/0" }
        ],
        "gateway": "192.168.1.1"
      }
    }'
EOF
```

Kubernetes communicates with admission webhooks using HTTPS, therefore we need to
create certificates and let Kubernetes CA sign them. Following script will create
such a certificate, ask Kubernetes to sign and once that is done, key and certificate
will be created as a Secret on Kubernetes API.

```shell
./hack/create-signed-cert.sh --app foo
```

In the next step, you can generate manifests for your admission webhook using
included script. Manifests then can be found under `_out/` directory.

```shell
CA_BUNDLE=$(kubectl get configmap -n kube-system extension-apiserver-authentication -o=jsonpath='{.data.client-ca-file}' | base64 | tr -d '\n')
./hack/render-manifests.sh --app foo --ca-bundle $CA_BUNDLE --image network-attachment-definition-pod-admission
kubectl apply -f _out/
```

Create pod that requests a network:

```shell
cat <<EOF | kubectl create -f -
apiVersion: v1
kind: Pod
metadata:
  name: samplepod
  annotations:
    k8s.v1.cni.cncf.io/networks: macvlan-conf
spec:
  containers:
  - name: samplepod
    command: ["/bin/bash", "-c", "sleep 2000000000000"]
    image: dougbtv/centos-network
EOF
```

## Configuration API

**TODO**, all possible configuration (select nad type, whitelist label**

## Development

- **TODO:**
- whoever wants to use this, there is no need to build it or ship images yourself
- however, if you want to, there are some commands that should help you
- make, test, test cluster

Build docker image with admission:

```shell
docker build -f cmd/admission/Dockerfile -t network-attachment-definition-pod-admission .
```

For easier deployment and functional testing, this projects ships a
[dind](https://github.com/kubernetes-sigs/kubeadm-dind-cluster) script. It
allows you to deploy simple Kubernetes cluster inside a container.

```shell
# start dind cluster
./dind-cluster.sh up

# use kubectl on the cluster
export PATH=${PWD}/.kubeadm-dind-cluster:${PATH}
kubectl get nodes

# push local docker image to the cluster
./dind-cluster.sh copy-image network-attachment-definition-pod-admission

# stop the cluster
./dind-cluster.sh down

# remove dind containers and volumes
./dind-cluster.sh clean
```

## TODO

- [x] single node dind cluster
- [x] script to get ca
- [x] script to generate cert, put it on kubernetes, sign it, generate secret (?)
- [x] script to generate all manifests from templates
- [x] extend the script to create rbac as well
- [x] basic server doing nothing
- [x] implement reading of requested networks
- [ ] implement reading of config map (monitor for latest changes, keep up to date (later))
- [ ] implement json templating
- [ ] deploy it in its own namespace and use namespaceSelector to blacklist it
- [ ] implement end to end tests
