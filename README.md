# provider-cloudinit

Crossplane provider for Cloud-init templating

## Example

```yaml
# TODO: ProviderConfig has no use in this provider and is not loaded by Config resources
# apiVersion: cloudinit.crossplane.io/v1alpha1
# kind: ProviderConfig
# metadata:
#  name: default
---
apiVersion: cloudinit.crossplane.io/v1alpha1
kind: Config
metadata:
  name: cloudinit
spec:
  writeCloudInitToRef:
    name: cloudinit
  forProvider:
    boundary: MIMEBOUNDARY
    part: # TODO: rename this parts?
    - content: |
        #!/bin/sh
        echo "Hello World, from provider-cloudinit"
    - content: |
        #cloud-config
        users:
        - default
        - name: yourusername
          gecos: Your Name
          sudo: ALL=(ALL) NOPASSWD:ALL
          ssh_authorized_keys:
            - ssh-rsa YOURKEY
    # TODO: demonstrate configMapKeyRef
    # TODO: implement and demonstrate secretKeyRef
---
apiVersion: equinixmetal something
kind: Device
metadata:
  name: device-with-cloudinit-userdata
spec:
  forProvider:
    operatingSystem: ubuntun_20_04
    userdata:
      # TODO: implement this in provider-equinix-metal
      valueFrom:
        kind: ConfigMap
        name: provider-cloudinit-configmap-foo
```

## Testing

`make run`
