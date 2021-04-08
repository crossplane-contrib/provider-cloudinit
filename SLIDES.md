---
marp: true
theme: gaia
paginate: true
class: invert
html: true
inlineSVG: true
backgroundColor: #183d54
---
<!-- _theme: default -->
<!-- _class: lead -->

<!-- _backgroundColor: #183d54 -->
<!-- _backgroundColor: rgba(0, 0, 0, 0)-->
<!-- _backgroundPosition-x: 0%-->
<!-- _backgroundPosition-y: 0%-->
<!-- _backgroundRepeat: repeat-->
<!-- _backgroundAttachment: scroll-->
<!-- _backgroundImage: linear-gradient(90deg, rgb(0, 211, 185) 0%, rgb(255, 203, 0) 100%)-->
<!-- _backgroundSize: auto-->
<!-- _backgroundOrigin: padding-box-->
<!-- _backgroundClip: border-box-->
![bg 90% right drop-shadow](https://cncf-branding.netlify.app/img/projects/crossplane/icon/color/crossplane-icon-color.png)
![bg 100% right drop-shadow](https://github.com/cncf/artwork/raw/master/other/kubecon-cloudnativecon/2021-eu-virtual/color/kubecon-eu-2021-color.svg)

# Using Cloud-init with Crossplane
##### provider-cloudinit

Marques Johansson | Equinix
<span style="font-size:80%">@displague | @equinixmetal</span>

![height:1em](https://metal.equinix.com/metal/images/logo/equinix-metal-full.svg) 

May 4, 2021
##### <!-- fit -->Crossplane Community Day Europe

---

# Cloud-Init

> Cloud-init is the industry standard multi-distribution method for cross-platform cloud instance initialization

Cloud-init is a service **within the OS** that handles "userdata" supplied through Cloud metadata or within the device.

Userdata can be provided in "cloud-config" format, `#!` scripts, multi-part mime, and can be gzip compressed.

_Alternatives include Ignition (*CoreOS) and Kickstart (RHEL)._

---

# Cloud-Init Usage

Cloud provider API fields:

```sh
curl -X POST -H "Content-Type: application/json" \
  -H "X-Auth-Token: $METAL_AUTH_TOKEN" \
  https://api.equinix.com/metal/v1/devices \
  -d '{"userdata":"#!/bin/sh\necho hello world", ...}"
```

Cloud-Config format

```yaml
#cloud-config
packages:
  - vim
```

---
# Why a Crossplane Provider?

Crossplane manages:
* services
* storage
* ...
* AND VMs and Bare Metal

---
# Why a Crossplane Provider?

Crossplane manages:
* services
* storage
* ...
* _and_ **VMs** and **Bare Metal** instances

## But how do we configure the instances?

---
# Configuring Instances

* Images
* Provider specific "formations"
* SSH <!-- idempotency and timing -->
* Userdata (with Cloud-Init features)
* iPXE URLs

Why not use simple Userdata?

* Content length restrictions
* Multiple configurations

---

# Why a Crossplane Provider?

#### <!-- fit --> There is no provider !?
#### <!-- fit --> There is no remote API !?
#### <!-- fit --> There are no "credentials" !?
#### <!-- fit --> There is no resulting status !?

---

# Why a Crossplane Provider?

![w:40% bg contain brightness:.6 sepia:50%](https://icons.veryicon.com/png/Movie%20%26%20TV/Futurama%20Vol.%201/Zoidberg.png)

## <!-- fit --> Why not?

---
# Why a Crossplane Provider?
## Why not an ad-hoc controller?

* Crossplane Installer and RBAC
* Crossplane Runtime
* Composable
* Future: Referencers as Cloud-Init template variables?

---

# Why a Crossplane Provider?

## Why not an XRD?

Compositions allow for marrying ConfigMaps and Managed Resources.

Cloud-init rendering needs string functions, concatenation, base64-encoding, compression.

We'll want the ability to append multiple ConfigMaps.

Some day, <https://github.com/crossplane/crossplane/pull/1705>.

---

# Syntax

```yaml
apiVersion: cloudinit.crossplane.io/v1alpha1
kind: Config
...
spec:
  writeCloudInitToRef:
    name: cloudinit
    namespace: default
    key: cloud-init
  forProvider:
    parts:
    - content: |
        #!/bin/sh
        echo "Hello World, from provider-cloudinit"
```

---

## <!-- fit --><span style="color:orange;text-shadow: #FC0 .025em 0 .1em;">Demo Time</span>

![bg 100%](https://upload.wikimedia.org/wikipedia/commons/d/de/Demolition_t_%282574171717%29.jpg)

---
# Alternatives

Is this pattern better than ...

* Managed provider native support for multiple ConfigMaps
<!--- 
* Device.spec.forProvider.userdataConfigMapRefs
  * move userdata fields to spec level. 
* reusable provider?
* Bake this type resource appending, base64 encoding, gzip'ing type into XP RT?
* use a selector to fetch all configmaps to join <https://github.com/crossplane/provider-aws/blob/master/apis/database/v1beta1/rdsinstance_types.go#L473-L477>. misses out on benefits of composition? potential for template replacements?
* when we have templating? referencers instead of templating

 --->
* Composition to create Userdata?
  <!-- patch from configmaps not yet supported -->
* Terraform Cloud-Init and Template providers as Crossplane providers?
  <!-- missing support for datasources? (are they all that different?) -->

no.

---

![w:50% bg sepia:20% blur:3px](https://www.terraform.io/assets/images/og-image-8b3e4f7d.png)
# Influences
## Terraform Providers

* Template Provider
  * `template_file` and `template_cloudinit_config` Resources
  * now available as `templatefile(path, vars)` in HCL
* CloudInit Provider
  * `cloudinit_config` Data Source


---

# Influences

## Crossplane Helm Provider

* Similar in-cluster design
* Similar ConfigMap and Secret handling

## Differences

* No Remote API!

---
<!-- _backgroundColor: >
<!-- _theme: gaia -->
<!-- _class: lead inverted -->

<!-- _theme: default -->
<!-- _class: lead -->
<!-- _backgroundColor: #183d54 -->
<!-- _backgroundColor: rgba(0, 0, 0, 0)-->
<!-- _backgroundPosition-x: 0%-->
<!-- _backgroundPosition-y: 0%-->
<!-- _backgroundRepeat: repeat-->
<!-- _backgroundAttachment: scroll-->
<!-- _backgroundImage: linear-gradient(90deg, rgb(0, 211, 185) 0%, rgb(255, 203, 0) 100%)-->
<!-- _backgroundSize: auto-->
<!-- _backgroundOrigin: padding-box-->
<!-- _backgroundClip: border-box-->
#### Contributions Welcome!  [displague/provider-cloudinit](https://git.io/JYAMz)

## Questions?
<!--- How _janky_ is this?
Let me know, let the maintainers know, inspire new features and capabilities in Crossplane.--->
**. . .**

Marques Johansson
<span style="font-size:.6em">Principal Software Engineer, Integrations
Developer Relations  @ Equinix Metal
<svg xmlns="http://www.w3.org/2000/svg" width="1em" height="1em" viewBox="0 0 24 24"><path style="fill: rgb(29,161,242); fill-rule: nonzero; opacity: 1;" d="M24 4.557c-.883.392-1.832.656-2.828.775 1.017-.609 1.798-1.574 2.165-2.724-.951.564-2.005.974-3.127 1.195-.897-.957-2.178-1.555-3.594-1.555-3.179 0-5.515 2.966-4.797 6.045-4.091-.205-7.719-2.165-10.148-5.144-1.29 2.213-.669 5.108 1.523 6.574-.806-.026-1.566-.247-2.229-.616-.054 2.281 1.581 4.415 3.949 4.89-.693.188-1.452.232-2.224.084.626 1.956 2.444 3.379 4.6 3.419-2.07 1.623-4.678 2.348-7.29 2.04 2.179 1.397 4.768 2.212 7.548 2.212 9.142 0 14.307-7.721 13.995-14.646.962-.695 1.797-1.562 2.457-2.549z"/></svg> **@displague** <svg width="1em" height="1em" viewBox="0 0 1024 1024" fill="none" xmlns="http://www.w3.org/2000/svg"><path fill-rule="evenodd" clip-rule="evenodd" d="M8 0C3.58 0 0 3.58 0 8C0 11.54 2.29 14.53 5.47 15.59C5.87 15.66 6.02 15.42 6.02 15.21C6.02 15.02 6.01 14.39 6.01 13.72C4 14.09 3.48 13.23 3.32 12.78C3.23 12.55 2.84 11.84 2.5 11.65C2.22 11.5 1.82 11.13 2.49 11.12C3.12 11.11 3.57 11.7 3.72 11.94C4.44 13.15 5.59 12.81 6.05 12.6C6.12 12.08 6.33 11.73 6.56 11.53C4.78 11.33 2.92 10.64 2.92 7.58C2.92 6.71 3.23 5.99 3.74 5.43C3.66 5.23 3.38 4.41 3.82 3.31C3.82 3.31 4.49 3.1 6.02 4.13C6.66 3.95 7.34 3.86 8.02 3.86C8.7 3.86 9.38 3.95 10.02 4.13C11.55 3.09 12.22 3.31 12.22 3.31C12.66 4.41 12.38 5.23 12.3 5.43C12.81 5.99 13.12 6.7 13.12 7.58C13.12 10.65 11.25 11.33 9.47 11.53C9.76 11.78 10.01 12.26 10.01 13.01C10.01 14.08 10 14.94 10 15.21C10 15.42 10.15 15.67 10.55 15.59C13.71 14.53 16 11.53 16 8C16 3.58 12.42 0 8 0Z" transform="scale(64)" fill="#1B1F23"/></svg></span>

![bg right drop-shadow](https://cncf-branding.netlify.app/img/projects/crossplane/icon/color/crossplane-icon-color.png)
<!--![bg 100% right drop-shadow](https://github.com/cncf/artwork/raw/master/other/kubecon-cloudnativecon/2021-eu-virtual/color/kubecon-eu-2021-color.svg)-->

---

![bg 100%](https://upload.wikimedia.org/wikipedia/commons/c/cd/Lenox_Globe_Dragons.png)

<!---
unorganized slide fodder

## final thoughts

* providers don't have to interact with APIs
  * atProvider / forProvider relevance
  * ProviderConfig relevance
    error: error validating "providerconfig.yaml": error validating data: ValidationError(ProviderConfig): missing required field "spec" in io.crossplane.cloudinit.v1alpha1.ProviderConfig; if you choose to ignore these errors, turn validation off with --validate=false

---

# decisions and questions

* draws from provider-helm
* DataKeySelector vs xpv1.Reference / xpv1.SecretReference
* up-2-date checks are full recomputes instead of api read/writes
* specname as remote client "id"
* credentialsSecretRef, writeSecretRefTo not needed in this case

* externalname any purpose
* crossplane runtime, angryjet provide a convenient interface
* more code than a terraform provider
* more functionality than a terraform provider
* when angryjet is hella-perturbed, can't build, misleading errors
  * generate by hand and look for errors

* if this is janky - tell me how to make it better, hopefully inspiring new features for crossplane

---

## todo and other thoughts

* copy setval from helm provider, use provider for templated string replacement
* Treate CloudConfig as a DSL, <https://github.com/juju/juju/blob/develop/cloudconfig/cloudinit/cloudinit.go>
  * packages: sshkeys: etc
* reading in secrets

---
## example integration

* change cloudconfig configmap
  * redeployed instance
  * what if other instances depend on values from this instance?
  * cyclical dependency loop?
    * solve via xrd

* configmap namespace, provider namespace

```
xrd e:
  xrd d:
   composition b:
    patch device.status.atProvider.ip_address to configmap

  xrd c (optional ip address):
    composition a:
       patch(required ip address) configmap to device.spec.forProvider.userdata
```
--->
