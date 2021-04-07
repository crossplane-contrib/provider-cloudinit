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
apiVersion: v1
kind: ConfigMap
metadata:
  name: foo
  namespace: default
data:
  foo: '#\!/bin/sh'
---
apiVersion: cloudinit.crossplane.io/v1alpha1
kind: Config
metadata:
  name: cloudinit
  namespace: default
spec:
  writeCloudInitToRef:
    name: cloudinit
    namespace: default
    key: cloud-init
  forProvider:
    boundary: MIMEBOUNDARY
    parts:
    - configMapKeyRef:
      name: foo
      namespace: default
      key: foo
      optional: true
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
    # TODO: demonstrate secretKeyRef
---
```

This configuration produces a ConfigMap (as named in the Config resource spec):

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: cloudinit
  namespace: default
data:
  cloud-init: "Content-Type: multipart/mixed; boundary=\"MIMEBOUNDARY\"\nMIME-Version:
    1.0\r\n\r\n--MIMEBOUNDARY\r\nContent-Transfer-Encoding: 7bit\r\nContent-Type:
    text/plain\r\nMime-Version: 1.0\r\n\r\n#\\!/bin/sh\r\n--MIMEBOUNDARY\r\nContent-Transfer-Encoding:
    7bit\r\nContent-Type: text/plain\r\nMime-Version: 1.0\r\n\r\n#!/bin/sh\necho \"Hello
    World, from provider-cloudinit\"\n\r\n--MIMEBOUNDARY\r\nContent-Transfer-Encoding:
    7bit\r\nContent-Type: text/plain\r\nMime-Version: 1.0\r\n\r\n#cloud-config\nusers:\n-
    default\n- name: yourusername\n  gecos: Your Name\n  sudo: ALL=(ALL) NOPASSWD:ALL\n
    \ ssh_authorized_keys:\n    - ssh-rsa YOURKEY\n\r\n--MIMEBOUNDARY--\r\n"
```

This configmap can then be consumed by `userdata` wanting resources that accept ConfigMaps:

```yaml
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
