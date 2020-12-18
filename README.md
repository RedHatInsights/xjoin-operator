XJoin (Cross Join) Operator
==============

Openshift operator that manages the [XJoin pipeline](https://platform-docs.cloud.paas.psi.redhat.com/backend/xjoin.html).
It currently manages the M2 version of XJoin,
i.e. it maintains the replication pipeline between HBI and ElasticSearch.
Modifications will be necessary to support M3 (joining data between applications).

## Development

### Setting up the development environment

1. Download and unpack [CodeReady Containers](https://developers.redhat.com/products/codeready-containers/overview)

1. Append the following line into `/etc/hosts`
    ```
    127.0.0.1 inventory-db xjoin-elasticsearch-es-default
    ```
   
1. Configure CRC to use 16G of memory
    ```
    ./crc config set memory 16384
    ```

1. Start CRC
    ```
    ./crc start
    ```

1. When prompted for a pull secret paste it (you obtained pull secret on step 1 when downloading CRC)

1. Log in to the cluster as kubeadmin (oc login -u kubeadmin -p ...)
   You'll find the exact command to use in the CRC startup log

1. Log in to https://quay.io/
   From Account settings download a kubernetes secret.
   This secret is used to pull quay.io/cloudservices images

1. Run the setup script. <pull-secret-name> is the value of the `metadata.name` field in the quay.io secret file.
    ```
    dev/setup.sh <pull-secret-name> <pull-secret-file-location>
    ```
   
1. Forward ports to ElasticSearch, the HBI DB, and Kafka Connect:
    ```
    dev/forward-ports.sh
    ```
   
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
    oc apply -f ../config/samples/xjoin_v1alpha1_xjoinpipeline.yaml
    ```
