# kubectl-jq

kubectl-jq is a [kubectl plugin](https://kubernetes.io/docs/tasks/extend-kubectl/kubectl-plugins/)
that works like `kubectl get`, but lets you filter the output with a
[jq](https://stedolan.github.io/jq/) expression.

You can of course pipe the output of `kubectl get -o json` into `jq`, but this saves you valuable
keystrokes. We also unpack lists by default, so you don't have to edit your jq program depending on
the output (`kubectl get` will return one item as a lone item of that type, but multiple items as a
`v1.List`).

## Installing

    go install github.com/jrockway/kubectl-jq/cmd/kubectl-jq@latest

Eventually this will be available through [krew](https://krew.sigs.k8s.io/), but I want to make sure
that this actually solves my problems first. Feedback is welcome if it solves (or doesn't solve)
your own problems.

In the mean time, releases are published on Github and you can also grab the binary, make it
executable, and install it.

Finally, you can `brew tap jrockway/tap; brew install jrockway/tap/kubectl-jq`. This works on Linux
and MacOS.

## Running

`kubectl-jq` works exactly like `kubectl get`, except it takes an extra argument; the jq program to
run:

```
$ kubectl get pods
NAME                                       READY   STATUS    RESTARTS   AGE
node-debugger-pool-ga20saarp-3ip2b-hchnr   1/1     Running   0          2d12h

$ kubectl get pods -o json
{
    "apiVersion": "v1",
    "items": [
        {
            "apiVersion": "v1",
            "kind": "Pod",
    ...

$ kubectl jq pods
{
    "apiVersion": "v1",
    "kind": "Pod",
    "metadata": {
    ...

$ kubectl jq pods '{name: .metadata.name, phase: .status.phase}'
{
    "name": "node-debugger-pool-ga20saarp-3ip2b-hchnr",
    "phase": "Running"
}

$ kubectl jq pods node-debugger-pool-ga20saarp-3ip2b-hchnr '{name: .metadata.name, phase: .status.phase}'
<same as above>
```

You can invoke it with `--help` to see what options can be supplied, but it's basically the same as
`kubectl get`. Things you expect to work like `--all-namespaces` or `--context` do work.

## Configuration

By default, if you have a file called `.jq` in your home directory, any functions you define there
will be available. (`jq` itself uses the same file.) You can specify other places to search for
modules, including these auto-loaded `.jq` files, with the `--jq-search-path` (comma-separated)
flag.

In mine, I have something like this:

```jq
def named(name): select(.metadata.name | test(name));
```

to work around the lack of tab-completion in kubectl plugins. You can then run something like
`kubectl jq pods 'named("foo") | ...'` to run `...` on any pods that contain the string `foo`.

## Recipes

I mostly use `jq` to extract things from secrets with the `@base64d` operator and see what port
number something has bound before I `port-forward` to it. But there are many things you can do.

-   Print the ports that pods have configured.

```
kubectl jq pod .spec.containers[]?.ports[]?
```

-   Decode a secret.

```
kubectl jq secret my-secret '.data |= with_entries(.value |= @base64d)' -o yaml
```

-   Print the exact contents of "filename.yaml" from a secret.

```
kubectl jq secret my-secret '.data |= with_entries(.value |= @base64d) | .data."filename.yaml"' -r
```

-   Print pods with containers that have restarted.

```
kubectl jq pod -A 'select(.status.containerStatuses[]?.restartCount > 0) | {namespace: .metadata.namespace, name: .metadata.name}'
```
