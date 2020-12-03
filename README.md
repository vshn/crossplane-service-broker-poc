# Crossplane Service Broker POC

POC Code to provide an [Open Service Broker](https://github.com/openservicebrokerapi/servicebroker) API which provisions
Redis and MariaDB instances via [crossplane](https://crossplane.io/).

## Usage

### Starting the broker

```console
export KUBECONFIG=/path/to/kubeconfig
OSB_USERNAME=test OSB_PASSWORD=TEST OSB_SERVICE_ID=id make run
```

### Testing

[eden](https://github.com/starkandwayne/eden) can be used to test the OSB integration.

```console
$ export SB_BROKER_URL=http://localhost:8080
$ export SB_BROKER_USERNAME=test
$ export SB_BROKER_PASSWORD=TEST
```

#### List Catalog

```console
$ eden catalog
Service     Plan          Free         Description
=======     ====          ====         ===========
redis-helm  redis-large   unspecified
~           redis-medium  unspecified
~           redis-small   unspecified

```

#### Provision Service

```console
$ export SB_INSTANCE=my-test-redis
$ eden provision -s redis-helm -p redis-small
```

#### Bind Service

```console
$ eden bind --instance my-test-redis
# output will contain a CLI to retrieve the credentials, execute that
$ eden credentials #...
```

### Custom APIs

This implementation contains a couple of custom APIs, not defined by the OSB spec.

#### Get endpoints

```console
# ensure to either export or replace the $INSTANCE_UUID variable:
$ curl 'http://localhost:8080/custom/service_instances/$INSTANCE_UUID/endpoint' -u test:TEST -v|jq
```


## Development

If the env var `KUBECONFIG` is set, it will be used to connect to downstream clusters instead of the actual provider config.
This helps to debug locally since downstream clusters are usually not accessible from a workstation.
