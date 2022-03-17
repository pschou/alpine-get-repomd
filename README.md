# Alpine Get Repo Meta Data


## To grab all of the current ALPINE keys:

```bash
wget -nc -np -r -nH --reject="index.html*" https://alpinelinux.org/keys/
wget -nc -np -r -nH --cut-dirs=3 -w 3 https://git.alpinelinux.org/aports/plain/main/alpine-keys/
```

