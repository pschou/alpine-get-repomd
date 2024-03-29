package main

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html"
)

var version = "test"

var keys_url *string
var indexFileName = "APKINDEX.tar.gz"
var lastUpdated = 0
var debug *bool

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Alpine Get RepoIndex, Version: %s\n\nUsage: %s [options...]\n\n", version, os.Args[0])
		flag.PrintDefaults()
	}

	inRepoPath := flag.String("repo", "latest-stable/main/x86_64", "Repo path to use in fetching")
	mirrorList := flag.String("mirrors", "MIRRORS.txt", "Mirror / directory list of prefixes to use")
	outputPath := flag.String("output", ".", "Path to put the APKINDEX.tar.gz file")
	// Another key repository: https://git.alpinelinux.org/aports/plain/main/alpine-keys/
	keys_url = flag.String("keysUrl", "https://alpinelinux.org/keys/", "Where to fetch current keys from")
	keysDir := flag.String("keysDir", "keys/", "Use keysDir for verifying signature")
	fetch_keys := flag.Bool("fetchkeys", false, "Fetch keys before downloading metadata")
	debug = flag.Bool("debug", false, "Turn on debugging")
	var timeout = flag.Duration("timeout", 5*time.Second, "HTTP Client Timeout")
	var secureCert = flag.String("client-cert", "", "Satellite repo, CERT for using PKI auth")
	var secureKey = flag.String("client-key", "", "Satellite repo, KEY for using PKI auth")
	var secureUser = flag.String("client-user", "", "Satellite repo, using basic USER auth")
	var securePass = flag.String("client-pass", "", "Satellite repo, PASS for USER")
	flag.Parse()

	http.DefaultClient.Timeout = *timeout
	if *secureCert != "" {
		if cert, err := tls.LoadX509KeyPair(*secureCert, *secureKey); err != nil {
			log.Fatal(err)
		} else {
			http.DefaultClient.Transport = &http.Transport{TLSClientConfig: &tls.Config{Certificates: []tls.Certificate{cert}}}
		}
	}

	mirrors := readMirrors(*mirrorList)
	repoPath := strings.TrimSuffix(strings.TrimPrefix(*inRepoPath, "/"), "/")

	if *fetch_keys {
		// 1) update pub keys in keys
		if *debug {
			fmt.Println("Updating keys in", *keysDir)
		}
		pullKeys(*keysDir)
	}

	// 2) use mirror list to fetch MIRRORS.txt if updated
	if *debug {
		fmt.Println("Loading mirror list", *mirrorList)
	}
	mirrors = pullMirrorList(mirrors)

	// 2a) pull last-updated
	if *debug {
		fmt.Println("Checking mirrors to find the lastest", *mirrorList)
	}
	lastUpdated = pullLastUpdated(mirrors)

	// Loop over mirrors to get the index files
	if *debug {
		fmt.Println("Looping over mirrors to get the index files")
	}
	var newestModTime *time.Time
	var newestPKG []byte
	var new_mirrors []string

	var mu sync.Mutex
	var wg sync.WaitGroup

	for j, mm := range mirrors {
		i := j
		u, err := url.Parse(mm)
		if err != nil {
			fmt.Println("Error using mirror", mm)
			continue
		}
		time.Sleep(70 * time.Millisecond)
		wg.Add(1)
		go func() {
			defer wg.Done()
			u.Path = strings.TrimSuffix(u.Path, "/")
			if *secureUser != "" {
				u.User = url.UserPassword(*secureUser, *securePass)
			}

			if *debug {
				fmt.Printf(" %d/%d) %s\n", i, len(mirrors), u.Redacted())
			}
			mirror := strings.TrimSuffix(u.String(), "/")
			indexPath := mirror + "/" + repoPath + "/" + indexFileName

			// 2b) pull APKINDEX.tar.gz
			start := time.Now()
			resp, err := http.DefaultClient.Get(indexPath)
			diff := time.Now().Sub(start)
			if err != nil {
				if *debug {
					log.Println("  ", i, "error connecting", err)
				}
				return
			}

			index, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				if *debug {
					log.Println("  ", i, "error getting file", err)
				}
				resp.Body.Close()
				return
			}
			resp.Body.Close()

			mu.Lock()
			defer mu.Unlock()

			zsig, zcontent := split_on_gzip_header(index)

			// 3) validate Index signature
			if mod_time, ok := verify(zsig, zcontent, *keysDir); ok {
				if *debug {
					fmt.Printf("   %d tar mod time = %+v\n", i, mod_time)
				}
				if newestModTime == nil || newestModTime.Before(mod_time) {
					if newestModTime != nil && *debug {
						fmt.Printf("   %d found newer %+v\n", i, mod_time)
					}
					newestModTime = &mod_time
					newestPKG = index
					new_mirrors = []string{mirror}
				} else if !newestModTime.After(mod_time) {
					new_mirrors = append(new_mirrors, fmt.Sprintf("%s\n# latency: %s", u.Redacted(), diff))
				}
			}
			// 3a) if sig fails next mirror

		}()
	}
	wg.Wait()

	if newestModTime != nil {
		dir := *outputPath //path.Join(*outputPath), repoPath)
		// 4) write ouput_dir/{inRepoPath}/APKINDEX.tar.gz
		err := ensureDir(dir)
		check(err)
		filename := path.Join(dir, indexFileName)

		if *debug {
			fmt.Println("  creating file:", filename)
		}
		fd, err := os.Create(filename)
		check(err)

		fd.Write(newestPKG)
		fd.Close()

		os.Chtimes(filename, *newestModTime, *newestModTime)

	}
}

func pullMirrorList(mirrors []string) []string {
	return mirrors
}

func pullLastUpdated(mirrors []string) int {
	return 0
}

func pullKeys(keysDir string) {
	resp, err := http.DefaultClient.Get(*keys_url)
	check(err)

	defer resp.Body.Close()

	doc, err := html.Parse(resp.Body)
	check(err)

	// Barrowed from https://stackoverflow.com/questions/29318672/parsing-list-items-from-html-with-go
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			for _, a := range n.Attr {
				if a.Key == "href" && a.Val != ".." && strings.HasSuffix(a.Val, ".pub") {
					keyName, err := url.QueryUnescape(a.Val)
					check(err)

					if *debug {
						fmt.Println("Pulling key for", keyName)
					}
					pullKeyByUrl(keyName, keysDir)

					break
				}
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}

	f(doc)
}

func pullKeyByUrl(keyName string, keysDir string) {
	if _, err := os.Stat(path.Join(keysDir, keyName)); errors.Is(err, os.ErrNotExist) {
		resp, err := http.DefaultClient.Get(strings.TrimSuffix(*keys_url, "/") + "/" + keyName)
		check(err)

		defer resp.Body.Close()

		err = ensureDir(keysDir)
		check(err)

		_, keyFileName := path.Split(keyName)
		fd, err := os.Create(path.Join(keysDir, keyFileName))
		check(err)

		defer fd.Close()

		pub_key, err := ioutil.ReadAll(resp.Body)
		check(err)

		fd.Write(pub_key)
	}
}

func readMirrors(mirrorFile string) []string {
	file, err := os.Open(mirrorFile)

	if err != nil {
		log.Fatal(err)
	}

	defer file.Close()

	var line string

	ret := []string{}
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line = strings.TrimSpace(scanner.Text())

		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}

		line = strings.TrimSuffix(line, "/")
		ret = append(ret, line)
	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}

	return ret
}

func verify(zsig []byte, zcontent []byte, pub_dir string) (time.Time, bool) {
	zbuf := bytes.NewBuffer(zsig)
	gzr, err := gzip.NewReader(zbuf)

	if err != nil {
		if *debug {
			log.Println("Error when gzunipping file", err)
		}
		return time.Time{}, false
	}

	tsig, err := ioutil.ReadAll(gzr)
	if err != nil {
		if *debug {
			log.Println("Error when reading file", err)
		}
		return time.Time{}, false
	}

	tbuf := bytes.NewBuffer(tsig)
	tr := tar.NewReader(tbuf)

	header, err := tr.Next()
	//fmt.Printf("tar header %+v\n", header)
	if err != nil {
		if *debug {
			log.Println("Error when parsing tar header", err)
		}
		return time.Time{}, false
	}

	pub_key_name := strings.ReplaceAll(header.FileInfo().Name(), ".SIGN.RSA.", "")

	pub_key_path := path.Join(pub_dir, pub_key_name)

	pub_key_fd, err := os.Open(pub_key_path)
	if err != nil {
		if *debug {
			log.Println("Error when opening public key", pub_key_path, err)
		}
		return time.Time{}, false
	}

	hash := sha1.Sum(zcontent)

	pub_key_pem, err := ioutil.ReadAll(pub_key_fd)
	if err != nil {
		if *debug {
			log.Println("Error when reading public key", pub_key_path, err)
		}
		return time.Time{}, false
	}

	pub_key, _ := pem.Decode(pub_key_pem)
	if pub_key == nil {
		fmt.Println("Invalid PEM Block")
		return time.Time{}, false
	}

	key, err := x509.ParsePKIXPublicKey(pub_key.Bytes)
	if err != nil {
		if *debug {
			log.Println("Error when parsing public key", pub_key_path, err)
		}
		return time.Time{}, false
	}

	pubKey := key.(*rsa.PublicKey)

	sig, err := ioutil.ReadAll(tr)
	if err != nil {
		if *debug {
			log.Println("Error when reading signature", err)
		}
		return time.Time{}, false
	}

	err = rsa.VerifyPKCS1v15(pubKey, crypto.SHA1, hash[:], sig)
	if err != nil {
		zcontent, _ = split_on_gzip_header(zcontent)
		hash = sha1.Sum(zcontent)
		b64 := base64.StdEncoding.EncodeToString(hash[:])
		fmt.Println(b64)

		err = rsa.VerifyPKCS1v15(pubKey, crypto.SHA1, hash[:], sig)
		if err != nil {
			if *debug {
				log.Println("Signature check failed", err)
			}
			return time.Time{}, false
		}
	}

	return header.ModTime, true
}

func split_on_gzip_header(data []byte) ([]byte, []byte) {
	gzip_header := []byte{0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00}
	//arr := []byte(gzip_header)

	pos := bytes.Index(data[1:], gzip_header) + 1
	if pos < 1 {
		return []byte{}, data
	}
	return data[:pos], data[pos:]
}

func check(e error) {
	if e != nil {
		log.Fatal(e)
	}
}
