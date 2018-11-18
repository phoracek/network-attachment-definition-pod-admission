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

Kubernetes communicates with admission webhooks using HTTPS, therefore we need to
create certificates and let Kubernetes CA sign them. Following script will create
such a certificate, ask Kubernetes to sign and once that is done, key and certificate
will be created as a Secret on Kubernetes API.

```shell
./hack/create-signed-cert.sh --service foo-admission-svc --secret foo-admission-secret --namespace default
```

## Configuration API

**TODO**, all possible configuration (select nad type, whitelist label**

## Development

- **TODO:**
- whoever wants to use this, there is no need to build it or ship images yourself
- however, if you want to, there are some commands that should help you
- make, test, test cluster

For easier deployment and functional testing, this projects ships a
[dind](https://github.com/kubernetes-sigs/kubeadm-dind-cluster) script. It
allows you to deploy simple Kubernetes cluster inside a container.

```shell
# start dind cluster
./dind-cluster.sh up

# use kubectl on the cluster
export PATH=${PWD}/.kubeadm-dind-cluster:${PATH}
kubectl get nodes

# stop the cluster
./dind-cluster.sh down

# remove dind containers and volumes
./dind-cluster.sh clean
```

## TODO

- [x] single node dind cluster
- [ ] script to get ca
- [x] script to generate cert, put it on kubernetes, sign it, generate secret (?)
- [ ] script to generate all manifests from templates
- [ ] basic server doing nothing
- [ ] implement reading of requested networks
- [ ] implement reading of config map (monitor for latest changes, keep up to date (later))
- [ ] implement json templating
- [ ] implement end to end tests
