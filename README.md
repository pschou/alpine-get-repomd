# Alpine Get Repo Meta Data

This is a Alpine repo metadata downloader, it will fetch the latest APKINDEX and then make a subset of LATEST\_MIRRORS.txt for downloading packages from.

## Examples
To grab the current ALPINE repo metadata:
```bash
./alpine-get-repomd -output out
```

To grab the current ALPINE repo metadata and latest keys:
```bash
./alpine-get-repomd -output out -fetchkeys
```

Example usage for downloading the repo into out/
```bash
$ ./alpine-get-repomd -debug -output out -mirrors MIRRORS.txt
CGO_ENABLED=0 go build -ldflags="-s -w -X main.version=0.1.20220317.1309" -o "alpine-get-repomd" main.go filelib.go
Loading mirror list MIRRORS2.txt
Checking mirrors to find the lastest MIRRORS2.txt
Looping over mirrors to get the index files
 0/10) http://dl-cdn.alpinelinux.org/alpine
  tar mod time = 2022-03-17 10:45:03 -0400 EDT
 1/10) http://uk.alpinelinux.org/alpine
  tar mod time = 2022-03-17 10:45:03 -0400 EDT
 2/10) http://dl-4.alpinelinux.org/alpine
  tar mod time = 2022-03-17 10:45:03 -0400 EDT
 3/10) http://dl-5.alpinelinux.org/alpine
  tar mod time = 2022-03-17 10:45:03 -0400 EDT
 4/10) http://mirror.yandex.ru/mirrors/alpine
  tar mod time = 2022-03-17 10:45:03 -0400 EDT
 5/10) http://mirrors.gigenet.com/alpinelinux
  tar mod time = 2022-03-17 06:53:07 -0400 EDT
 6/10) http://mirror1.hs-esslingen.de/pub/Mirrors/alpine
  tar mod time = 2022-03-17 10:45:03 -0400 EDT
 7/10) http://mirror.leaseweb.com/alpine
  tar mod time = 2022-03-17 06:53:07 -0400 EDT
 8/10) http://mirror.fit.cvut.cz/alpine
  tar mod time = 2022-03-17 10:45:03 -0400 EDT
 9/10) http://alpine.mirror.far.fi
  tar mod time = 2021-11-18 08:28:04 -0500 EST
  creating file: out/APKINDEX.tar.gz
  creating file: out/LATEST_MIRRORS.txt
```

Usage:
```bash
$ ./alpine-get-repomd -h
Alpine Get RepoIndex, Version: 0.1.20...

Usage: ./alpine-get-repomd [options...]

  -debug
        Turn on debugging
  -fetchkeys
        Fetch keys before downloading metadata
  -keysDir string
        Use keysDir for verifying signature (default "keys/")
  -keysUrl string
        Where to fetch current keys from (default "https://alpinelinux.org/keys/")
  -mirrors string
        Mirror / directory list of prefixes to use (default "MIRRORS.txt")
  -output string
        Path to put the APKINDEX.tar.gz file (default ".")
  -repo string
        Repo path to use in fetching (default "latest-stable/main/x86_64")
  -timeout duration
        HTTP Client Timeout (default 5s)
```
