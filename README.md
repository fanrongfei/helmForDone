# helm3 plugin for drone
Drone plugin for Helm3.
Helm Version: 3.5.0  
Kubectl Version: 1.19.7
## Drone settings

Example:

```yaml
- name: deploy app
  image: bitsbeats/drone-helm3
  settings:
    kube_api_server: kube.example.com
    kube_token: { from_secret: kube_token }

    chart: ./path-to/chart(或采用远程库myrepo/exampleChart)
    release: release-name
    namespace: namespace-name
    timeout: 20m
    helm_repos:
      - myrepo=http://127.0.0.1:8080/chartrepo/helm=admin=123456=yes(添加自己的私有库)
    envsubst: true
    values:
      - app.environment=awesome
      - app.tag=${DRONE_TAG/v/}
      - app.commit=${DRONE_COMMIT_SHA}
```

