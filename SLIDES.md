# Provider Cloud-Init

## provisioning managed services
  
* what about tranditional vm services? baremetal?
  * how do you configure them?
    * provider ssh? idempotency?
    * userdata. limitations?
      * cloud-init.
        * provider?

## terraform provider

* limitations

## why a provider? why not controller?

* advantages?
* overkill?

* why not an xrd?

## features of the configmap provider

* draws from provider-helm
* DataKeySelector vs xpv1.Reference / xpv1.SecretReference
* up-2-date checks are full recomputes instead of api read/writes
* specname as remote client "id"

## example integration

* change cloudconfig configmap
  * redeployed instance
  * what if other instances depend on values from this instance?
  * cyclical dependency loop?
    * solve via xrd
## final thoughts

* providers don't have to interact with APIs
  * atProvider / forProvider relevance
* crossplane runtime, angryjet provide a convenient interface
* more code than a terraform provider
* more functionality than a terraform provider

## todo

* copy setval from helm provider, use provider for templated string replacement
* Treate CloudConfig as a DSL, https://github.com/juju/juju/blob/develop/cloudconfig/cloudinit/cloudinit.go
  * packages: sshkeys: etc
