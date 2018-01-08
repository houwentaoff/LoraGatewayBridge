package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	log "github.com/sirupsen/logrus"
)

/*
3个服务器
1. 无验证
2. 单方面 服务器验证
3. 双向验证
*/
func handler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w,
		"Hi, This is danxiang an example of https service in golang!")
}

type myhandler struct {
}

func (h *myhandler) ServeHTTP(w http.ResponseWriter,
	r *http.Request) {
	fmt.Fprintf(w,
		"Hi, shuangxiang This is an example of http service in golang!\n")
}
func shuangx() {
	pool := x509.NewCertPool()
	caCertPath := "ca.crt"

	caCrt, err := ioutil.ReadFile(caCertPath)
	if err != nil {
		log.Warningln("ReadFile err:", err)
		return
	}
	pool.AppendCertsFromPEM(caCrt)

	s := &http.Server{
		Addr:    ":8081",
		Handler: &myhandler{},
		TLSConfig: &tls.Config{
			ClientCAs:  pool,
			ClientAuth: tls.RequireAndVerifyClientCert,
		},
	}

	log.Infoln("start https shuangxiang 8081")
	err = s.ListenAndServeTLS("server.crt", "server.key")
	if err != nil {
		log.Println("ListenAndServeTLS err:", err)
	}
}
func danx() {
	http.HandleFunc("/", handler)
	log.Infoln("start https danxiang 8082")
	err := http.ListenAndServeTLS(":8082", "server.crt",
		"server.key", nil)
	if err != nil {
		log.Fatalln("start failed!", err)
	}
}
func main() {
	go shuangx()
	go danx()
	for {
		time.Sleep(time.Second)
	}
}
