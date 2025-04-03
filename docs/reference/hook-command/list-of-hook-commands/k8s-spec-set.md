(hook-command-k8s-spec-set)=
# `k8s-spec-set`

```
Usage: k8s-spec-set [options] --file <core spec file> [--k8s-resources <k8s spec file>]

Summary:
set k8s spec information

Options:
--file  (= -)
    file containing pod spec
--k8s-resources  (= )
    file containing k8s specific resources not yet modelled by Juju

Details:
Sets configuration data to use for k8s resources.
The spec applies to all units for the application.
```