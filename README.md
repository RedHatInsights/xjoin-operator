XJoin (Cross Join) Operator
==============

Openshift operator that manages the [XJoin pipeline](https://clouddot.pages.redhat.com/docs/dev/services/xjoin.html).
It currently manages version 1 of XJoin,
i.e. it maintains the replication pipeline between HBI and ElasticSearch.
Modifications will be necessary to support [version 2](#version-2) (joining data between applications).

A XJoin pipeline is defined by the
[XJoinPipeline custom resource](./config/crd/bases/xjoin.cloud.redhat.com_xjoinpipelines.yaml).
It indexes the hosts table of the HBI database into an ElasticSearch index.
A Debezium Kafka Connector is used to read from the HBI database's replication slot.
An ElasticSearch connector is used to index the hosts.
Kafka Connect transformations are performed on the ElasticSearch connector to prepare the host records to be indexed.
An ElasticSearch pipeline is used to transform the JSON fields on a host prior to being indexed.

![Architecture](./docs/architecture.png "XJoin Operator Architecture")

The operator is responsible for:

- management of an ElasticSearch index, alias, and pipeline
- management of Debezium (source) and ElasticSearch (sink) connectors in a Kafka Connect cluster (using Strimzi)
- management of a Kafka Topic in a Kafka cluster (using Strimzi)
- management of the HBI replication slot. The Debezium connector should manage this. The operator ensures there are no
  orphaned replication slots.
- periodic validation of the indexed data
- automated recovery (e.g. when the data becomes out-of sync)

## Implementation

The operator defines two controllers that reconcile a XJoinPipeline

* [PipelineController](./controllers/xjoinpipeline_controller.go) which manages all the resources
  (connectors, elasticsearch resources, topic, replication slots) and handles recovery
* [ValidationController](./controllers/validation_controller.go) which periodically compares the data in the
  ElasticSearch index with what is stored in HBI to determine whether the pipeline is valid

## Development

### Setting up the development environment using Clowder

1. Install the latest version of [bonfire](https://github.com/RedHatInsights/bonfire)
2. Set up a local Kubernetes environment. Known to work with the following:
    - [CodeReady Containers](https://developers.redhat.com/products/codeready-containers/overview)
    - [MiniKube](https://minikube.sigs.k8s.io/docs/start/)

3. Configure Kubernetes to use at least 16G of memory and 6 cpus. This is known to work, although you can try with less.
    ```
    ./crc config set memory 16384
    ./crc config set cpus 6
    ```
    ```
    minikube config set cpus 6
    minikube config set memory 16384
    ```

4. If using minikube, use the kvm2 driver. The docker driver is known to cause problems.
   ```
   minikube config set driver kvm2
   ```

5. Start Kubernetes
    ```
    ./crc start
    ```
    ```
    minikube start
    ```

6. If using CRC
    - When prompted for a pull secret paste it (you obtained pull secret on step 1 when downloading CRC)
    - Log in to the cluster as kubeadmin (oc login -u kubeadmin -p ...)
      You'll find the exact command to use in the CRC startup log

7. Login to https://quay.io and https://registry.redhat.io
    - `docker login -u=<quay-username> -p="password" quay.io`
    - `docker login https://registry.redhat.io`
    - For MacOS, do the following to place the creds in .docker/config.json, which are stored
      in `"credsStore": "desktop"|"osxkeystore"` and are not available for pulling images from private repos.
        1. `docker logout quay.io`
        2. `docker logout registry.redhat.io`
        3. Remove the "credStore" block from .docker/config.json.
        4. `docker login -u=<quay-username> -p="password" quay.io`
        5. `docker login https://registry.redhat.io`

        - NOTE: Manually creating the `.docker/config.json` and adding `"auth": base64-encoded username:password` does
          not work.

8. Do one of the following
    - Append the following line into `/etc/hosts`
        ```
        127.0.0.1 inventory-db host-inventory-db.test.svc xjoin-elasticsearch-es-default.test.svc connect-connect-api.test.svc xjoin-elasticsearch-es-http kafka-kafka-0.kafka-kafka-brokers.test.svc
        ```
    - Install and run [kubefwd](https://github.com/txn2/kubefwd)
      ```
      sudo -E kubefwd svc -n test --kubeconfig ~/.kube/config -m 8080:8090 -m 8081:8091
      ```

9. `./dev/setup-clowder.sh`

### Forward ports

To access the services within the Kubernetes cluster there is a script to forward ports to each of the useful services:

```bash
./dev/forward-ports-clowder.sh
```

### Reset the development environment

The Openshift environment can be deleted with this script:

```bash
./dev/teardown.sh
```

Afterwards, the environment can be setup again without restarting Kubernetes via `dev/setup.sh`.

### Running the operator locally

With the cluster set up it is now possible to install manifests and run the operator locally.

1. Install CRDs
    ```
    make install
    ```

1. Run the operator
    ```
    make run ENABLE_WEBHOOKS=false
    ```

1. Finally, create a new pipeline
    ```
    kubectl apply -f ../config/samples/xjoin_v1alpha1_xjoinpipeline.yaml -n test
    ```

There is also `make delve` to debug the operator. After starting the Delve server process, connect to it with a Delve
debugger.

### Running the operator locally via OLM

This is useful when testing deployment related changes. It's a little cumbersome for everyday development because an
image needs to be built by app-interface and pushed to the cluster for each change.

- To deploy the operator via locally OLM run

```bash
./dev/install.operator.locally.sh
```

- To uninstall the OLM deployed operator run

```bash
./dev/uninstall.operator.locally.sh
```

### Running the operator locally via OLM using operator-sdk run bundle

This is more convenient than using the app-interface build because the build is done locally then pushed to quay.io.
[More info](https://sdk.operatorframework.io/docs/olm-integration/testing-deployment)

```bash
docker login -u=$QUAY_USERNAME -p $QUAY_PASSWORD
./dev/install.operator.with.operator.sdk.sh
```

`./dev/uninstall.operator.with.operator.sdk.sh ` to uninstall.

### Running tests

- The tests require an initialized Kubernetes environment. See [Setting up the development environment](#development).
- Before running the tests, make sure the operator is not already running on your machine or in the cluster.
  ```
  kubectl scale --replicas=0 deployments/xjoin-operator-controller-manager -n xjoin-operator-system
  ```
- They can be executed via `make test`.
- There is also `make delve-test` to run the tests in debug mode. Then `delve` can be used to connect to the test run.
- The tests take a while to run. To whitelist one or a few tests, prepend `It` with an F. e.g.
  change `It("Creates a connector...` to `FIt("Creates a connector...) {`
- Sometimes when the test execution fails unexpectedly it will leave orphaned projects in kubernetes.
  Use `dev/cleanup.projects.sh` to remove them.

## Version 2

### Local development

The xjoin-api-gateway does not yet have an official build, so a custom entry needs to be added to
`~/.config/bonfire/config.yaml`.

```
- name: xjoin-api-gateway
  components:
  - name: xjoin-api-gateway
    host: local
    repo: /home/chris/dev/projects/active/xjoin-api-gateway
    path: clowdapp.yaml
```

The simplest way to create a complete local development environment is via the `./dev/setup_clowder.sh true` script.
This
script will install resources in the `test` namespace. The true flag will install the additional dependencies required
for
xjoin.v2. See the [version 1 development](#development) section for details on setting up minikube and running the
script.

After setting up a kubernetes environment, the xjoin-operator code can be run like this:

```
make run ENABLE_WEBHOOKS=false
```

Now that the xjoin-operator is running, it is time to create the k8s custom resources that define how the data is
streamed from the database into Elasticsearch. Once the custom resources are defined, the xjoin-operator will start
reconciling each resource and create the components necessary to stream the data.

```
kubectl apply -f config/samples/xjoin_v1alpha1_xjoindatasource.yaml -n test
kubectl apply -f config/samples/xjoin_v1alpha1_xjoinindex.yaml -n test
```

These commands create two k8s resources (XJoinIndex and XJoinDataSource) which then create many more k8s resources.
The xjoin k8s resources are defined in the [api/v1alpha1](api/v1alpha1) directory.

### Running the tests

The xjoin.v2 tests utilize mocks, so they don't require kubernetes or any other services to run.

```
make generic-test
```

### Troubleshooting tips

This is a good order of where to look when the data is not syncing. When troubleshooting data sync issues, always query
the Elasticsearch index directly instead of querying the GraphQL API.

1. Verify the Kafka and Kafka Connect pods are running
2. Check the logs of the xjoin-operator
3. Look at the status of each of the following resources via `kubectl get -o yaml <resource name>`:
   `XJoinIndex`, `XJoinIndexPipeline`, `XJoinDataSource`, `XJoinDataSourcePipeline`, `KafkaConnector`
4. Check the logs of Kafka Connect via
    ```
    kubectl logs -f -l app.kubernetes.io/instance=connect --all-containers=true`
    ```
5. Check the logs of Kafka via
   ```
   kubectl logs -f -l app.kubernetes.io/instance=kafka --all-containers=true
   ```
6. Check the logs of xjoin-core via
    ```
    kubectl logs -f -l xjoin.index=xjoin-core-xjoinindexpipeline-hosts --all-containers=true
    ```
7. Check if messages are on the datasource and index topics by using kcat to consume each topic from the beginning
   ```
   kcat -b localhost:29092 -C -o beginning -f '%h\n%s\n\n\n' -t xjoin.inventory.1663597318878465070.public.hosts
   ```

### xjoin.v2 code walkthrough

The CRDs are contained in the [api/v1alpha1](api/v1alpha1) directory.

| Custom Resource Definition | Description                                                                                                                                                                                                                                                                                                                                                                                                       | Created By |
|----------------------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|------------|
| XJoinDataSource            | This defines a single data source (e.g. a database table). This is created by the user.                                                                                                                                                                                                                                                                                                                           | Human      |
| XJoinDataSourcePipeline    | This defines the pipeline for a DataSource. Each DataSource can have multiple DataSourcePipelines. e.g. there will be two DataSourcePipeline's for a single DataSource when the DataSource is being refreshed. The old DataSource will continue to be used by applications until the new DataSource is in sync. At that point, the old DataSource is deleted and the new DataSource will be used by applications. | Code       |
| XJoinIndex                 | This defines an Index that is composed of one or more DataSources. The Index defines how the data is indexed into Elasticsearch. This is created by the user.                                                                                                                                                                                                                                                     | Human      |
| XJoinIndexPipeline         | This defines the pipeline for an Index. This is similar to the XJoinDataSourcePipeline where each Index can have multiple IndexPipelines.                                                                                                                                                                                                                                                                         | Code       |
| XJoinIndexValidator        | This defines the validator for an Index. The validator periodically compares the data in each DataSource with the data in the Index. The validator is responsible for updating the status of the Index and each DataSource used by the Index.                                                                                                                                                                     | Code       |

The entrypoint to the reconcile loop for each Custom Resource Definition is in a separate file in the top level of the [controllers](controllers) directory. e.g. the `Reconcile` method in the [xjoindatasource_controller](controllers/xjoindatasource_controller.go) file is the entrypoint for a DataSource.

Almost all the remaining business logic is contained in the [controllers](controllers) directory. The other directories are mostly boilerplate and related to deployments/builds.

Both the DataSourcePipeline and the IndexPipeline manage multiple resources to construct a pipeline for the data to flow. Throughout the code these resources are referred to as a `Component` and they are managed by a `ComponentManager`. More details can by found in the [controllers/components/README.md](controllers/components/README.md).

The code for the top level resources (XJoinDataSource and XJoinIndex) can be found in the [controllers/datasource](controllers/datasource) and the [controllers/index](controllers/index) directory.

There are many different parameters and sources of parameters across the operator. These are handled by the [ConfigManager](controllers/config/manager.go). The parameters for the xjoin.v2 resources are defined in [controllers/parameters](controllers/parameters).