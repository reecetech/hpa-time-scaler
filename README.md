# HPA Time Scaler

## Overview

The HPA Time Scaler is a tool that allows developers to dynamically scale the minimum number of pods or replicas in their HPA resources based on the time of day. It provides a flexible way to pre-emptively set a minimum availabilty during peak times that works along side the current HPA metrics.

## Installation

This is expected to run along side your applications specifically as a cronjob. To consume it, simply create a cronjob.yaml within your deployment and reference this image!

### Cronjob

```YAML
apiVersion: v1
kind: ConfigMap
metadata:
  name: hpa-time-scaler-cron-config
data:
  # -----------------------------------------------------------------------------------------------
  # HPA TIME SCALER CONFIG
  # -----------------------------------------------------------------------------------------------
  SCALE_UP_TIME: "08:00"
  SCALE_DOWN_TIME: "13:30"
  TIMEZONE: "Australia/Melbourne"
  SCALE_UP_REPLICAS: "5"
  SCALE_DOWN_REPLICAS: "2"
  HPA_NAME: "hpa-resource-name"
---
apiVersion: batch/v1
kind: CronJob
metadata:
  name: hpa-time-scaler-cron
  labels:
    app: hpa-time-scaler-cron
spec:
  successfulJobsHistoryLimit: 1
  schedule: "1,31 * * * 1-5"
  timeZone: "Australia/Melbourne"
  concurrencyPolicy: Forbid
  jobTemplate:
    spec:
      template:
        metadata:
          name: hpa-time-scaler-cron
          labels:
            app: hpa-time-scaler-cron
        spec:
          serviceAccountName: hpa-time-scaler-sa
          containers:
            - name: hpa-time-scaler-cron
              image: ghcr.io/reecetech/hpa-time-scaler:1.0.0
              imagePullPolicy: IfNotPresent
              tty: true
              env:
                - name: NAMESPACE
                  valueFrom:
                    fieldRef:
                      fieldPath: metadata.namespace
              envFrom:
                - configMapRef:
                    name: hpa-time-scaler-cron-config
          restartPolicy: OnFailure
```

### Role and RoleBinding

```YAML
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: hpa-time-scaler_rolebinding
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: hpa-time-scaler_role
subjects:
# Add all the ServiceAccounts that need to be bound to this Role
- kind: ServiceAccount
  name: hpa-time-scaler-sa

---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: hpa-time-scaler_role
rules:
- apiGroups:
  - autoscaling/v2
  - autoscaling
  resources:
  - horizontalpodautoscalers
  # Optional: scope down to specific HPA resource
  # resourcenames:
  # - hpa-resource-name
  verbs:
  - get
  - list
  - patch
  - update
- apiGroups:
  - events.k8s.io
  resources:
  - events
  verbs:
  - create
```

## Configuration

| Environment Variable | Description                                                                           | Default Value |
|----------------------|---------------------------------------------------------------------------------------|---------------|
| SCALE_UP_TIME        | The time at which to scale up the minimum number of replicas.                         | 05:00         |
| SCALE_DOWN_TIME      | The time at which to scale down the minimum number of replicas.                       | 18:00         |
| TIMEZONE             | The timezone to use for time calculations.                                            | UTC           |
| SCALE_UP_REPLICAS    | The number of minimum replicas to scale up to during the specified scale up time.     | 2             |
| SCALE_DOWN_REPLICAS  | The number of minimum replicas to scale down to during the specified scale down time. | 1             |
| HPA_NAME             | The name of the HPA resource to scale.                                                | Required      |
| NAMESPACE            | The namespace of the HPA resource.                                                    | Required      |
| LOCAL_RUN            | Uses local .kube/config over in-cluster config.                                       | false         |
