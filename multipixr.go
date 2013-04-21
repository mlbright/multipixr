package main

import (
	"code.google.com/p/goauth2/oauth"
	drive "code.google.com/p/google-api-go-client/drive/v2"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sort"
)

var (
	defaultCacheFile = filepath.Join(os.Getenv("HOME"), ".multipixr-request-token")
	cachefile        = flag.String("cachefile", defaultCacheFile, "Authentication token cache file")
)

var fileset = make(map[string]int64)

func visit(path string, f os.FileInfo, err error) error {
	if !f.IsDir() {
		fileset[path] = f.Size()
	}
	return nil
}

// A data structure to hold a key/value pair.
type Pair struct {
	Key   string
	Value int64
}

// A slice of Pairs that implements sort.Interface to sort by Value.
type PairList []Pair

func (p PairList) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
func (p PairList) Len() int           { return len(p) }
func (p PairList) Less(i, j int) bool { return p[i].Value < p[j].Value }

// A function to turn a map into a PairList, then sort and return it. 
func sortMapByValue(m map[string]int64) PairList {
	p := make(PairList, len(m))
	i := 0
	for k, v := range m {
		p[i] = Pair{k, v}
		i++
	}
	sort.Sort(p)
	return p
}

type Message struct {
	ClientId     string
	ClientSecret string
	Path         string
}

func main() {

	file, e := ioutil.ReadFile("./multipixr.json")
	if e != nil {
		fmt.Printf("File error: %v\n", e)
		os.Exit(1)
	}

	var jsonBlob = []byte(string(file))
	var msg Message
	e = json.Unmarshal(jsonBlob, &msg)
	if e != nil {
		fmt.Println("error:", e)
		os.Exit(1)
	}

    fmt.Println("Pictures path ", msg.Path)
    
	config := &oauth.Config{
		ClientId:     msg.ClientId,
		ClientSecret: msg.ClientSecret,
		Scope:        drive.DriveScope,
		AuthURL:      "https://accounts.google.com/o/oauth2/auth",
		TokenURL:     "https://accounts.google.com/o/oauth2/token",
		TokenCache:   oauth.CacheFile(*cachefile),
	}

	transport := &oauth.Transport{Config: config}

	if token, err := config.TokenCache.Token(); err != nil {
		err = authenticate(transport)
		if err != nil {
			log.Fatalln("authenticate:", err)
		}
	} else {
		transport.Token = token
	}

	service, err := drive.New(transport.Client())
	if err != nil {
		log.Fatal(err)
	}

	root := msg.Path
	err = filepath.Walk(root, visit)
	if err != nil {
		panic(err)
	}

	paths := sortMapByValue(fileset)

	for _, pair := range paths {
		path := pair.Key

		size := pair.Value
		goFile, err := os.Open(path)
		if err != nil {
			log.Fatalf("error opening %q: %v", path, err)
		}
		filename := filepath.Base(path)

		fmt.Printf("Uploading %v", path)
		driveFile, err := service.Files.Insert(&drive.File{Title: filename}).Media(goFile).Do()

		if err != nil {
			log.Printf("Got drive.File, err: %#v, %v", driveFile, err)
			fmt.Println("Could not upload ", path)
			closeerr := goFile.Close()
			if closeerr != nil {
				fmt.Println("Could not close file", path)
			}
			continue
		} else {
			fmt.Printf(" done: %v\n", size)
		}

		err = goFile.Close()
		if err != nil {
			fmt.Println("Could not close file", path)
		} else {
			err = os.Remove(path)
			if err != nil {
				fmt.Println("Could not delete ", path)
			} else {
				fmt.Println("Deleted ", path)
			}
		}
	}

}
