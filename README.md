# k8s-site-tinkerbell (WIP)

## Prerequsite


### Generate cerficate


Change the file `server-csr.in.json`


```bash

# cat server-csr.in.json

{

  "CN": "tinkerbell",

  "hosts": [

    "tinkerbell.sv3.es.equinix.com",

    "tinkerbell.registry",

    "tinkerbell.tinkerbell",

    "tinkerbell",

    "localhost",

    "registry.sv3.es.equinix.com",

    "tink.tinkerbell.svc",

    "10.20.3.131",

    "10.20.3.130",

    "10.20.3.128",

    "10.20.3.129",

    "10.20.3.133",

    "127.0.0.1"

  ],

  "key": {

    "algo": "rsa",

    "size": 2048

  },

  "names": [

    {

      "L": "onprem"

    }

  ]

}

```


Run the script to genreate cert files


```bash

bash-3.2$ ./gencerts.sh

```


The following files will be generated


```bash

bash-3.2$ ls -l *.pem

-rw-r--r--  1 kewang  staff  2713 Sep 16 14:51 bundle.pem

-rw-------  1 kewang  staff  1679 Sep 16 14:51 ca-key.pem

-rw-r--r--  1 kewang  staff  1200 Sep 16 14:51 ca.pem

-rw-------  1 kewang  staff  1675 Sep 16 14:51 server-key.pem

-rw-r--r--  1 kewang  staff  1513 Sep 16 14:51 server.pem

```

## Copy OSIE files to node02 /data directory


In this deployment, we pined nginx on node02, so we only copy OSIE files into node02 /data directory



# Deploy tinkerbell


> Before start deploy tinkerbell microservice, you need look at the values.yaml file in each mircoservices folder and replace the variables which works for your enviroment.


Deploy PostgresDB


```bash

helm install tink-db . -n tinkerbell --set postgresql.username=tinkerbell,postgresql.password=tinkerbell,postgresql.database=tinkerbell

```


Depoly tink server


```bash

 helm install tink k8s-site-tink -n tinkerbell

```


Deploy tink cli


```bash

helm install tink-cli k8s-site-tink-cli -n tinkerbell

```


Deploy boots


```bash

helm install boots k8s-site-boots -n tinkerbell

```


Deploy hegel


```bash

 helm install hegel k8s-site-hegel -n tinkerbell

```


Deploy nginx


```bash

 helm install nginx k8s-site-nginx -n tinkerbell

```


Deploy registry


```bash

helm install registry k8s-site-registry -n tinkerbell

```


Deploy pdns-recursor


pdns-recursor is hosted in packet private registry, so need create a image-pull-secret.


```bash

kubectl apply -f image-secret.yaml -n tinkerbell

```


```bash

helm install pdns-recursor k8s-site-pdns-recursor -n tinkerbell

```


# Prepare workflow action images


```bash

[admin@sv15-ems-sitectl-node01 Unified-workflow]$ export REGISTRY_HOST=172.23.17.130

[admin@sv15-ems-sitectl-node01 Unified-workflow]$ cat create_images.sh | envsubst

#!/bin/sh


docker build -t 172.23.17.130/ubuntu:base 00-base/

docker push 172.23.17.130/ubuntu:base


docker build -t 172.23.17.130/disk-wipe:v1 01-disk-wipe/ --build-arg REGISTRY=172.23.17.130

docker push 172.23.17.130/disk-wipe:v1


docker build -t 172.23.17.130/disk-partition:v1 02-disk-partition/ --build-arg REGISTRY=172.23.17.130

docker push 172.23.17.130/disk-partition:v1


docker build -t 172.23.17.130/windows:v1 03-install-windows/ --build-arg REGISTRY=172.23.17.130

docker push 172.23.17.130/windows:v1


docker build -t 172.23.17.130/install-root-fs:v1 03-install-root-fs/ --build-arg REGISTRY=172.23.17.130

docker push 172.23.17.130/install-root-fs:v1


docker build -t 172.23.17.130/install-grub:v1 04-install-grub/ --build-arg REGISTRY=172.23.17.130

docker push 172.23.17.130/install-grub:v1


docker build -t 172.23.17.130/cloud-init:v1 05-cloud-init/ --build-arg REGISTRY=172.23.17.130

docker push 172.23.17.130/cloud-init:v1


docker build -t 172.23.17.130/post-install:v1 06-post-install/ --build-arg REGISTRY=172.23.17.130

docker push 172.23.17.130/post-install:v1


docker build -t 172.23.17.130/reboot:v1 07-reboot/ --build-arg REGISTRY=172.23.17.130

docker push 172.23.17.130/reboot:v1


docker build -t 172.23.17.130/deprovision:v1 99-deprovision/ --build-arg REGISTRY=172.23.17.130

docker push 172.23.17.130/deprovision:v1

```


```bash

docker pull fluent/fluent-bit:1.3

docker image tag 2708a59c01a8 172.23.17.130/fluent-bit:1.3

docker push 172.23.17.130/fluent-bit

```


```bash

docker pull quay.io/tinkerbell/tink-worker:latest

docker tag af9645b3a013 172.23.17.130/tink-worker:latest

docker push 172.23.17.130/tink-worker:latest

```
