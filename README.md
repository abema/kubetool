# kubetool

kubetool is the tool that supports utility tasks which kubectl doesn't have.

## Install

You have to install `kubectl` on your machine since kubetool uses kubectl command to execute tasks.

```
go get github.com/abema/kubetool
```

## Features

`kubetool help` to see all commands.

### List replication controllers

```
kubetool rc
```

Output example
```
  NAME    REPLICAS  IMAGE       VERSION
  nginx   2/2       abema/nginx 1.9.1
  mysql   2/2       abema/mysql latest
```

### List pods

```
kubetool pods
```

Output example
```
     NAME      STATUS   R   POD IP     NODE IP    IMAGE       VERSION
  nginx-6k8hq  Running  0  10.8.0.4   10.78.0.2   abema/nginx 1.9.1
  nginx-s3vj6  Running  0  10.8.1.3   10.78.0.5   abema/nginx 1.9.1
  mysql-zm3iu  Running  0  10.8.2.7   10.78.0.9   abema/mysql latest
  mysql-gdmvb  Running  0  10.8.1.4   10.78.0.5   abema/mysql latest
```


### Reload pods

Reload all pods in RC. This is done by destroying all pods one by one.

```
kubetool reload nginx
```

Just reload 1 pod. This is useful when testing new image before reloading all pods.

```
kubetool reload nginx --1
```

### Update image version of RC

Patch image version of RC container definition.
ex) `image: nginx:1.9.1` -> `image: nginx:1.9.2`

```
kubetool update nginx 1.9.2
```

with reload
```
kubetool update nginx 1.9.2 --reload
```

with reload only 1 pod
```
kubetool update nginx 1.9.2 --reload --1
```

### Fix version

Fix container images which has different from RC they depends. This commands is
similar to reload, but only destroy old pods.

```
kubetool fix-version nginx
```


