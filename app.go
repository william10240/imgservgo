package main

import (
	"fmt"
	"github.com/go-redis/redis"
	"github.com/google/uuid"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"path"
	"time"
)

var photoPath string
var rds *redis.Client

const httpUri = "127.0.0.1:8080"

const photoFolder = "photos"

const rdsAddr = "127.0.0.1:6379"
const rdsDb = 0
const rdsExpire = 300

func u(w http.ResponseWriter, r *http.Request) {
	reader, err := r.MultipartReader()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	uuid := uuid.NewString()
	dst, _ := os.Create(path.Join(photoPath, uuid))
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}

		_, err = io.Copy(dst, part)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	defer dst.Close()

	w.Write([]byte(uuid))
}
func p(w http.ResponseWriter, r *http.Request) {
	// 取参数
	uid := r.URL.Query().Get("i")
	tp := path.Join(photoPath, uid)

	// 判断参数
	if uid == "" || !isExist(tp) {
		files, err := ioutil.ReadDir(photoPath)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		rand.Seed(time.Now().UnixNano())
		uid = files[rand.Intn(len(files))].Name()
	}
	w.Header().Set("X-mem-key", uid)

	// 响应
	res, err := rds.Get(uid).Bytes()
	if err == nil {
		rds.Expire(uid, time.Duration(rdsExpire)*time.Second)
		w.Header().Set("X-mem-cache", "HIT")
		w.Write(res)
	} else {
		fp := path.Join(photoPath, uid)
		rs, err := ioutil.ReadFile(fp)
		if err != nil {
			w.Header().Set("X-mem-cache", "MISS")
			http.Error(w, err.Error(), http.StatusInternalServerError)
		} else {
			err := rds.Set(uid, rs, time.Duration(rdsExpire)*time.Second).Err()
			//err := rds.Set(uid, rs, 0).Err()
			if err != nil {
				fmt.Println(err)
			}
			w.Header().Set("X-mem-cache", "NONE")
			w.Write(rs)
		}
	}

}

func main() {

	rds = redis.NewClient(&redis.Options{Addr: rdsAddr, DB: rdsDb})
	_, err := rds.Ping().Result()
	if err != nil {
		fmt.Println(err)
	}

	ph, err := os.Getwd()
	if err != nil {
		fmt.Println(err)
		return
	}

	photoPath = path.Join(ph, photoFolder)

	http.Handle("/", http.RedirectHandler("/p", 302))
	http.Handle("/favicon.ico", http.NotFoundHandler())
	http.HandleFunc("/u", u)
	http.HandleFunc("/p", p)
	err = http.ListenAndServe(httpUri, nil)
	if err != nil {
		fmt.Println(err)
	} else {
		fmt.Println("started")
	}
}

func isExist(path string) bool {
	_, err := os.Stat(path)
	if err != nil {
		if os.IsExist(err) {
			return true
		}
		if os.IsNotExist(err) {
			return false
		}
		fmt.Println(err)
		return false
	}
	return true
}
