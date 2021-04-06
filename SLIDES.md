# Provider Cloud-Init

## provisioning managed services

(inspiration)
typical uses

atypical uses
  
* what about traditional vm services? baremetal?
  * how do you configure them?
    * provider ssh? idempotency? (existing exploration in tbs/rawkode)
    * userdata. limitations?
      * cloud-init.
        * provider?
## Alternatives

device.userdata native support for configmaps
custom composition to create device.userdata?
patch from configmaps not yet supported
terraform cloudinit and template provider as crossplane providers?
  missing support for datasources? (are they all that different?)

## terraform provider

* limitations

## why a provider? why not controller?

* advantages?
* overkill?

* why not an xrd?
  * maybe someday? maybe just add support for functions (via lambda hooks or hcl / other dsl), i need a function to take input configmaps, apply templates, concatenate and gzip.. all local operations
    https://github.com/crossplane/crossplane/pull/1705


## features of the configmap provider

* draws from provider-helm
* DataKeySelector vs xpv1.Reference / xpv1.SecretReference
* up-2-date checks are full recomputes instead of api read/writes
* specname as remote client "id"
* credentialsSecretRef, writeSecretRefTo not needed in this case

## example integration

* change cloudconfig configmap
  * redeployed instance
  * what if other instances depend on values from this instance?
  * cyclical dependency loop?
    * solve via xrd

* configmap namespace, provider namespace

xrd e:
  xrd d:
   composition b:
    patch device.status.atProvider.ip_address to configmap

  xrd c (optional ip address):
    composition a:
       patch(required ip address) configmap to device.spec.forProvider.userdata

## final thoughts

* providers don't have to interact with APIs
  * atProvider / forProvider relevance
* externalname any purpose
* crossplane runtime, angryjet provide a convenient interface
* more code than a terraform provider
* more functionality than a terraform provider
* when angryjet is hella-perturbed, can't build, misleading errors
  * generate by hand and look for errors

if this is janky - tell me how to make it better, hopefully inspiring new features for crossplane

## todo

* copy setval from helm provider, use provider for templated string replacement
* Treate CloudConfig as a DSL, https://github.com/juju/juju/blob/develop/cloudconfig/cloudinit/cloudinit.go
  * packages: sshkeys: etc

* reading in secret