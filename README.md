kube-graffiti
=============

*kube-graffiti* is a tagger, labeller, annotator, patcher and blocker - at its core it's a convenient way of creating kubernetes mutating webhooks and modifying kubernetes objects on-the-fly as they are either created or updated.  It allows kubernetes cluster admins to modify arbitary objects without needing access to lots of different source files, e.g. kubernetes manifests, or helm charts/values, etc.  It is very useful for enabling or configuring various kubernetes services, such as [Kiam](https://github.com/uswitch/kiam), [Istio](https://istio.io/docs/setup/kubernetes/sidecar-injection/#automatic-sidecar-injection), [Network Policy](https://kubernetes.io/docs/concepts/services-networking/network-policies/) or anything else where it is useful to add labels or annotations.  You write rules that match against the kubernetes apis, versions and resources, object labels or field contents and then specify a payload, which could be the addition or deletion of lables or annotations, a specfic JSON patch, or just block the object from being created/updated.

**A few example use-cases:-**

1. Label specific namespaces (or all) to enable [kiam policies](https://github.com/uswitch/kiam) to be applied to pods (for granting AWS api credentials and access).
2. Label specific namespaces and/or pods to enable [Istio side-car injection](https://istio.io/docs/setup/kubernetes/sidecar-injection/#automatic-sidecar-injection).
3. Add "name" labels to namespaces, e.g. for using the namespace name within namespace selectors for [Network Policy](https://kubernetes.io/docs/concepts/services-networking/network-policies/) or [Admission Registration](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/) (webhooks).
4. Add ownership annotations to groups of objects in a similar way that security teams like to have tags on AWS resources.
5. Block certain deployments containing a specific docker image or other deployment specifications.

*kube-graffiti* can paint **any** kubernetes object that contains metadata (which I think is possibly all of them at the time of writing this readme).  It uses its own rules to determine whether to paint an object, or not, and to specify what to add.  It works by registering a number of [mutating webhooks](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/) (one per rule) with the kubernetes apiserver and matches incoming new or updated object requests before generating a json patch containing the desired additions if the rule matched.  It needs and expects to be running within a kubernetes cluster and must have been given adequate rbac permissions (see "kubernetes rbac access" section below). 

TL/DR Examples
--------------

Here's a few examples of the kinds of things that you might want *kube-graffiti* to do for you: -

**Labelling all namespaces except 'kube-system', 'kube-public' and 'default' for Istio side-car injection**
```
server:
  namespace: kube-graffiti
  service: kube-graffiti
rules:
- registration:
    name: namespace-enable-istio-injection
    targets:
    - api-groups:
      - ""
      api-versions:
      - v1
      resources:
      - namespaces
    failure-policy: Ignore
  matchers:
    label-selectors:
    - "name notin (kube-system,kube-public,default)"
  payload:
    additions:
      labels:
        istio-injection: enabled
```
*note* - 'name' and 'namespace' metadata fields can be used as though they are labels in our label-selectors.

**Annotating specific Namespaces to Enable Kiam**
```
server:
  namespace: kube-graffiti
  service: kube-graffiti
rules:
- registration:
    name: namespace-allows-kiam
    targets:
    - api-groups:
      - ""
      api-versions:
      - v1
      resources:
      - namespaces
    failure-policy: Ignore
  matchers:
    field-selectors:
    - "metadata.name=team-a"
    - "metadata.name=team-b"
    - "metadata.name=team-c"
  payload:
    additions:
      annotations:
        iam.amazonaws.com/permitted: ".*"
```
*note1* - a list of matcher label or field selectors are evaluated as an logical **OR**, and comma separated selectors within a single selector is treated as a logical **AND**

**Add a 'name' label to each namespace (i.e. useful for native kubernetes label selectors)**
```
server:
  namespace: kube-graffiti
  service: kube-graffiti
rules:
- registration:
    name: add-name-label-to-namespaces
    targets:
    - api-groups:
      - ""
      api-versions:
      - v1
      resources:
      - namespaces
    failure-policy: Ignore
  payload:
    additions:
      labels:
        name: '{{ index . "metadata.name" }}'
```
*note1* - a rule without any matcher section will match **ALL**
*note2* - addition values can be golang templates (but not keys)

**Add ownership labels to certain objects in the 'Mobile' team's namespace**

```
server:
  namespace: kube-graffiti
  service: kube-graffiti
rules:
- registration:
    name: add-name-label-to-namespaces
    targets:
    - api-groups:
      - ""
      api-versions:
      - v1
      resources:
      - namespaces
    failure-policy: Ignore
  payload:
    additions:
      labels:
        name: '{{ index . "metadata.name" }}'
- registration:
    name: magic-mobile-team-ownership-annotations
    targets:
    - api-groups:
      - "*"
      api-versions:
      - "*"
      resources:
      - pods
      - deployments
      - services
      - jobs
      - ingresses
    namespace-selector: name = mobile-team
    failure-policy: Ignore
  payload:
    additions:
      labels:
        owner: "Stephanie Jobs"
        security-zone: "alpha"
        contact: "mobileteam@mycorp.com"
        wiki: "http://wiki.mycorp.com/mobile-team"
```
*note1* - this uses the namespace-selector within the registration to pass only objects within the 'mobile-team' namespace to the *kube-graffiti* webhook.
*note2* - the 'add-name-label-to-namespaces' rule has been added to provide the required name label on the namespace.
*note3* - there are other ways of matching namespaces, such as using a label-selector or field-selector in the matcher rule, and these will work, but will result in more objects being passed through the *kube-graffiti* webhooks for evaluation, which can impact on cluster performance.

**Blocking an deployment, pod, job**

Note, there are other ways to do this - you should take a look at [ImagePolicyWebhook](https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/#imagepolicywebhook).  This demonstrates what you could do, remembering that you have api, version, resource-type, namespace-selector, label-selector and field-selectors to use to pin the object you want to target using the value of just about **any** field.

```
- registration:
    name: block-specific-deploy-in-mobile-team
    targets:
    - api-groups:
      - "*"
      api-versions:
      - "*"
      resources:
      - pods
      - deployments
    namespace-selector: name = mobile-team
    failure-policy: Ignore
  matchers:
    field-selectors:
    - spec.template.spec.containers.0.image=nginx
  payload:
    block: true
      labels:
```

*note1* - field selectors do not allow spaces around the '=' whereas label and namespace selectors are completely happy with them..

Configuration
-------------

*kube-graffiti* requires a configuration file in either yaml, json, toml or hcl format (depending on your preference) and will by default look for it at the path "/config.{yaml,json,toml,hcl}" - you can use the --config command line parameter to change it.

**Webhook Server Configuration**

See configuration example in testing/configmap.yaml

Because *kube-graffiti* runs as a kubernetes mutating webhook, it must trust TLS connections from the kubernetes apiserver and vice-versa the apiserver must trust it when delegating admission requests to it.  You must create a ca, server certificate and private key for it, (you could use the **gencerts.sh** script in the 'testing' folder).  These certificates are best placed into a kubernetes secret (see testing/webhook-tls-secret.yaml) and then mounted into the *kube-graffiti* pod as shown in testing/deploy.yaml deployment example.

By default, *kube-graffiti* will look for certificate at the following paths (files have no dot extension): -

* /ca-cert
* /server-cert
* /server-key

The kubernetes apiserver must be told where to find *kube-graffiti* so you must always specify the namespace and service where *kube-graffiti* has been installed in the 'server' section.  

*kube-grafffiti* has the following server configuration options and default values :-

```
log-level: info
check-existing: false
health-checker:
  port: 8080
  path: /healthz
server:
  port: 8443
  company-domain: acme.com
  namespace: ""
  service: ""
  ca-cert-path: /tls/ca-cert
  cert-path: /tls/server-cert
  key-path: /tls/server-key
```

You must specify values for "server.namespace" and "server.service" but you can omit any of the settings that you want to leave at their default settings.

Rules
-----

You need to have at least one rule in your configuration and can scale to as many rules that you want (and are willing to scale the kube apiserver and your *kube-graffiti* deployment to support).

A graffiti rule is made up of three parts:-

* **registration** - responsible for registering the rule as a mutating webhook.
* **matcher** - responsible for matching/evaluating the object to decide whether we paint it or not.
* **payload** - contains the changes that you want to make to the object

The rules are validated at start up and *kube-graffiti* will fail-fast if it finds any problems, check the logs to make sure you haven't entered any invalid selectors, labels or annotations.

**Registration**

```
registration:
    name: magic-mobile-team-ownership-annotations
    targets:
    - api-groups:
      - ""
      api-versions:
      - "v1"
      resources:
      - pods
    namespace-selector: name = mobile-team
    failure-policy: Fail
```

The goal of the registration is instruct the kubernetes apiserver which kubernetes objects to send through to *kube-graffiti* when they are created or updated, and we want to be as narrow/specific as possible for performance.

Each rule requires a unique **name**, which is converted into a URL path that is registered with the kubernetes apiserver routes back to *kube-graffiti* using the 'server.namespace' and 'server.service settings'.  *kube-graffiti* uses the path to match the incoming admission request against the correct rule.

Each registration contains a list of **targets** which are tuples of 'api-groups', 'api-versions' and 'resources' that identify which kubernetes objects we want to delegate to this rule.  They match in the same way that rules match in [kubernetes RBAC Roles](https://kubernetes.io/docs/reference/access-authn-authz/rbac/#referring-to-resources), except that 'verbs' is not used.  You can use the api-group "" to denote the core kubernetes group (i.e. namespaces, pods, secrets, services etc.) and you can also use "&ast;" as wild-cards (warning: use carefully as it is easy to make **everything** route through this rule).  You can specify lists of targets so you target a large number of objects without having to resort to using the wildcards "&ast;".

Each rule can contain a single **namespace-selector** which can be used to further narrow a registration to a set of namespaces that match this selector.  The namespace-selector is a kubernetes [label selector](https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/) and so I find it useful to include a *graffiti-rule* that adds a name label to my namespaces so that it can be used in namespace-selectors like this one.

**Matchers**

```
  matchers:
    label-selectors:
    - "name in (app-a,app-b,app-c)"
    field.selectors:
    - "spec.serviceAccountName=bob"
    boolean-operator: AND
```

Matchers take the incoming object and apply boolean logic to decide whether or not we will paint it with our additions.  You can use [kubernetes label selectors](https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#label-selectors), [field selectors](https://kubernetes.io/docs/concepts/overview/working-with-objects/field-selectors/) or a combination of the two.

**WARNING - please note that field selectors will not correctly match if you put spaces around the '='s: -**
```
x 'field = value' - WILL NOT match
✔ 'field=value'- WILL match
```

Both field and label selectors allow you to use commas ',' to specify multiple expressions which are AND'ed together to form the result, e.g.

```
  matchers:
    label-selectors:
    - "name=appa,team=mobile"
```

In the selector above the name AND team labels must match for the result to be true.  You can also specify multiple selectors that are OR'ed to form the result: -

```
  matchers:
    label-selectors:
    - "name=appa"
    - "team=mobile"
```

In the selector above name OR team labels must match for the result to be true.

Field selectors are not as flexible as label selectors and can not use set based matching, e.g. in or notin (x,y,z), which are available to label selectors.  They can, however, match **any** field in the source object.  The structure of the source object is flattened to a string map using dots and numbers to represent structure and lists.  For example, the following partial pod object:-

```
apiVersion: v1
kind: Pod
metadata:
  labels:
    app: kube-graffiti
    pod-template-hash: "4062789086"
  name: kube-graffiti-84b6cdf4db-tgn4x
  namespace: kube-graffiti
  ownerReferences:
  - apiVersion: extensions/v1beta1
    blockOwnerDeletion: true
    controller: true
    kind: ReplicaSet
    name: kube-graffiti-84b6cdf4db
    uid: 6bb0796d-aae0-11e8-90fb-0800272e64d7
  resourceVersion: "88145"
  selfLink: /api/v1/namespaces/kube-graffiti/pods/kube-graffiti-84b6cdf4db-tgn4x
  uid: 6bb1beaa-aae0-11e8-90fb-0800272e64d7
spec:
  containers:
  - args:
    - --config
    - /config/graffiti-config.yaml
    - --log-level
    - debug
    env:
    - name: GRAFFITI_LOG_LEVEL
      value: debug
    - name: GRAFFITI_CHECK_EXISTING
      value: "false"
    image: kube-graffiti:dev
    imagePullPolicy: Never
```

will be flattened to the following string map: -

```
apiVersion = v1
kind = Pod
metadata.labels.app = kube-graffiti
metadata.labels.pod-template-hash = "4062789086"
metadata.name = kube-graffiti-84b6cdf4db-tgn4x
metadata.namespace = kube-graffiti
metadata.ownerReferences.0.apiVersion = extensions/v1beta1
metadata.ownerReferences.0.blockOwnerDeletion = true
metadata.ownerReferences.0.controller = true
metadata.ownerReferences.0.kind = ReplicaSet
metadata.ownerReferences.0.name = kube-graffiti-84b6cdf4db
metadata.ownerReferences.0.uid = 6bb0796d-aae0-11e8-90fb-0800272e64d7
metadata.resourceVersion = "88145"
metadata.selfLink = /api/v1/namespaces/kube-graffiti/pods/kube-graffiti-84b6cdf4db-tgn4x
metadata.uid = 6bb1beaa-aae0-11e8-90fb-0800272e64d7
spec.containers.0.args.0 = --config
spec.containers.0.args.1 = /config/graffiti-config.yaml
spec.containers.0.args.2 = --log-level
spec.containers.0.args.3 = debug
spec.containers.0.env.0.name = GRAFFITI_LOG_LEVEL
spec.containers.0.env.0.value = debug
spec.containers.0.env.1.name = GRAFFITI_CHECK_EXISTING
spec.containers.0.env.1.value = "false"
spec.containers.0.image = kube-graffiti:dev
spec.containers.0.imagePullPolicy = Never
```

Unfortunately, at this time, neither label or field selectors support regex matching, as kubernetes extends these features then graffiti will gain them.

By default, both label-selectors AND field-selectors must match the object, *where they are specified*, for the result to be true.  This means that the result is effectively an AND when both selectors are set and an OR if only one selector is (with unset one evaluating to false).  If you omit both matchers then the result will **always** be true (this means anything matching the registration rule will always be painted).  You can change the logical operator used to combine results of the label and field selectors using the boolean-operator setting, from the default "AND" to "OR" or "XOR".  I have no idea of a real-world use-case for XOR but I think that OR may prove useful to someone.

**Payload**

The payload section allows you to: -

* add labels or annotations by specifying **additions**
* delete labels or annotations by specifying **deletions**
* provide your own json patch to the object with **json-patch**
* block the object with **block**

Each graffiti rule must contain multiple label/annotations additions or deletions, a single json-patch or a single block.  You **can not** mix a combination of labels/annotations changes, json-patches and block. 

**Additions**

```
  payload:
    additions:
      labels:
          istio-injection: enabled
      annotations:
          iam.amazonaws.com/permitted: ".*"
```

*kube-graffiti* supports the adding of lists of labels and annotations.  All additions must conform to the corresponding syntax of either labels or annotations.  The above example adds both a label and an annotation.

The value of a label or annotation can contain [golang template](https://golang.org/pkg/text/template/) annotations which allows a limited construction of values from the fields of the source object.  The flattened object map (that was described in the 'Matchers' section and which is a golang map[string]string) is available as the context to the template and can be referrenced using golang template's 'index' function.

Here's an example of combining a pods name with its uid in order to create a sort of asset tag kind of thing:-

```
  payload:
    additions:
      labels:
        asset-tag: 'pod/prod/k8s/{{ index . "metadata.namespace"}}/{{ index . "metadata.name" }}/{{ index . "metadata.uid" }}'
```

**Deletions**

```
  payload:
    deletions:
      labels:
      - istio-injection
      annotations:
      - iam.amazonaws.com/permitted: ".*"
```

We all make mistakes, especically when given a tool that can spray lots of new labels and annotations all over your kubernetes objects!  Deletions are a list of either label or annotation keys that you would like to remove from the object.  They get processed after any additions are applied and they are useful for applying against existing objects.  It is perfectly fine to have rules with only additions, deletions or a mixture of the two.

**Block**

Under certain circumstances it *might* be convenient to use kube-graffiti to block the creation/update of certain objects.  It has to be said that [kubernetes RBAC](https://kubernetes.io/docs/reference/access-authn-authz/rbac/) is absolutely the **right** way of limiting who can do what in your clusters, and if you want to limit the amount of something then [resource quotas](https://kubernetes.io/docs/concepts/policy/resource-quotas/) are what you want.  But, given that kube-graffiti has a rich collection of targetting and selectors, you may find it useful for temporarily blocking a bad-actor or errant process from running amok whilst you work out a better solution!

In this arbitary example we'll choose to stop anyone creating or updating any namespaces: -

```
rules:
- registration:
    name: no-more-namespaces
    targets:
    - api-groups:
      - ""
      api-versions:
      - v1
      resources:
      - namespaces
    failure-policy: Ignore
  payload:
    block: true
```

When you try to create a namespace (with this rule running) you'll see your request refused: -

```
$ ks create namespace please
Error from server (InternalError): Internal error occurred: admission webhook "no-more-namespaces.acme.com" denied the request: blocked by kube-graffiti rule: no-more-namespaces
```

**JSON-Patch (advanced)**

Want to update something in an object other than the either the labels or annotations?  You can do it by specifying your own [json patch](http://jsonpatch.com/) which can alter **any** part of the object you desire - which could be altering container images, changing secrets, environment variables, perhaps patching a deployment to add anti-affinity rules, or certain tolerations ... there's a long list of possible uses (and abuses!)  This is a low-level, powerful and rather dangerous feature so I would encourage careful testing of your rules containing json-patches in safe environment before you try them on any cluster that you care about!

```
rules:
- registration:
    name: remove-all-labels
  payload:
    json-patch: "[ { \"op\": \"delete\", \"path\": \"/metadata/labels\" } ]"
    additions:
      labels:
        added: label
```

kubernetes RBAC rules
---------------------

*kube-graffiti* needs (as a minimum) the following rbac permissions: -

* read the configmap 'extension-apiserver-authentication' in the 'kube-system' namespace
* create, delete 'mutatingwebhookconfigurations'
* list namespaces - used for the health-check

The following kubernetes objects configure this basic access, assuming that you choose to run kube-graffiti in its own namespace 'kube-graffiti': -

**roles**
```
apiVersion: rbac.authorization.k8s.io/v1beta1
kind: Role
metadata:
  name: read-apiserver-authentication
  namespace: kube-system
  labels:
    app: kube-graffiti
rules:
  - apiGroups:
      - ""
    resources:
      - "configmaps"
    resourceNames: 
      - "extension-apiserver-authentication"
    verbs:
      - get
      - list
---
apiVersion: rbac.authorization.k8s.io/v1beta1
kind: ClusterRole
metadata:
  name: manage-mutating-webhooks
  namespace: kube-system
  labels:
    app: kube-graffiti
rules:
  - apiGroups:
      - admissionregistration.k8s.io
    resources:
      - mutatingwebhookconfigurations
    verbs:
      - get
      - create
      - delete
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: list-namespaces
  labels:
    app: kube-graffiti
rules:
  - apiGroups:
      - ""
    resources:
      - namespaces
    verbs:
      - get
      - list
```

**role bindings**
```
apiVersion: rbac.authorization.k8s.io/v1beta1
kind: ClusterRoleBinding
metadata:
  name: kube-graffiti-manage-mutating-webhooks
  labels:
    app: kube-graffiti
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: manage-webhooks
subjects:
  - kind: ServiceAccount
    name: kube-graffiti
    namespace: kube-graffiti
---
apiVersion: rbac.authorization.k8s.io/v1beta1
kind: RoleBinding
metadata:
  name: read-apiserver-authentication
  namespace: kube-system
  labels:
    app: kube-graffiti
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: manage-mutating-webhooks
subjects:
  - kind: ServiceAccount
    name: kube-graffiti
    namespace: kube-graffiti
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: kube-graffiti-list-namespaces
  labels:
    app: kube-graffiti
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: list-namespaces
subjects:
  - kind: ServiceAccount
    name: kube-graffiti
    namespace: kube-graffiti
```

**service account**
```
apiVersion: v1
kind: ServiceAccount
metadata:
  name: kube-graffiti
  namespace: kube-graffiti
  labels:
    app: kube-graffiti
```

This is the minimal level of access required for *kube-graffiti* to operate as a mutating webhook.  If you want it to be able to update existing entires (not yet available) then you will need to give *kube-graffiti* **update** permissions on **all** objects that you want it to be able to modify.

Checking Existing Objects
-------------------------

kube-graffiti will check its rules against all the objects in kubernetes upon startup if you set --check-existing flag or set the environment variable GRAFFITI_CHECK_EXISTING=true.  It will interate once through your rules, applying them against all the existing objects in kubernetes which match.  It tries to be a good kubernetes citizen by looking up resources in batches (100 by default) and by caching namespace lookups (used when rules contain namespace selectors).

The rules behave as they would when using them in the mutating webhook, such as giving you the ability to use wildcards "&ast;" in the targetting of API Groups, Versions and Resources, but with subtley different behavoir around versions.  First, I would strongly recommend you use a wildcard for API Version for all of your rules unless you absolutely have to target a specific version of a resource (in the webhook).  Because kubernetes always stores your resources in the preferred version for that resource, it does not make sense to target an existing object with a rule **unless** the rules specifically lists the same preffered resource version (or is a wildcard "&ast;").  This means that is *is* possible to create rules which target non-prefferred versions in the webhook but will not target existing objects.

Example of good practice regarding matching versions: -

```
    targets:
    - api-groups:
      - ""
      - "appsv1"
      api-versions:
      - "*"
      resources:
      - pods
      - deployments
      - services
      - jobs
      - ingresses
```

or

```
    targets:
    - api-groups:
      - ""
      api-versions:
      - "*"
      resources:
      - pods
      - services
      - jobs
      - ingresses
    - api-groups:
      - "appsv1"
      api-versions:
      - "*"
      resources:
      - deployments
```

Contributing
------------

Submit a PR to this repository, following the [contributors guide](http://https://github.com/HotelsDotCom/kube-graffiti/CONTRIBUTING.md).