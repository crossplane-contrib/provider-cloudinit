# provider-cloudinit

Crossplane provider for Cloud-init templating

## Example

```yaml
apiVersion: equinixmetal something
kind: Device
metadata:
  name: device-with-cloudinit-userdata
spec:
  forProvider:
    operatingSystem: ubuntun_20_04
    userdata:
      valueFrom:
        kind: ConfigMap
        name: provider-cloudinit-configmap-foo
-- whatever that syntax is for saying load it from a configmap


apiVersion: cloudinit something
kind: Config
metadata:
  name: provider-cloudinit-configmap-foo
spec:
  forProvider:
    gzip: false
    mimeBoundary: MIME -- maybe this doesn't matter, don't bother
    part
    - contentType: "text/x-shellscript"
      content: |
      #!/bin/sh
    -  contentType: "text/cloud-config"
       valueFrom:
        kind:ConfigMap
        name: reused-userdata
      someWayToInterpollate:
        varA: value -- can this come from another crossplane resource?
```
