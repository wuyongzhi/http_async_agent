package main

import (
	"io/ioutil"
	"log"
	"net/http"
	"strings"
    "os"
    "io"
    "bytes"
)

type ProxyHostInfo struct {
	Scheme string
}

var ProxyHostInfoMap = map[string]*ProxyHostInfo{
	"api.jpush.cn": &ProxyHostInfo{
		Scheme: "https",
	},
}



const UserAgent string = "HttpAsyncAgent"

var logRequest, logError *log.Logger
var WriteRequestLog bool = true

func handler(w http.ResponseWriter, req *http.Request) {

    //调整URL
    req.RequestURI = ""
    req.URL.Host = req.Host
    proxyHostInfo := ProxyHostInfoMap[req.Host]
    if proxyHostInfo != nil {
        req.URL.Scheme = proxyHostInfo.Scheme
    } else {
        req.URL.Scheme = "http"
    }


    bodyBytes, err := ioutil.ReadAll(req.Body)
    newBodyReader := bytes.NewReader(bodyBytes)

    reqTarget, err := http.NewRequest(req.Method, req.URL.String(), newBodyReader)
    if err != nil {
        logError.Println(err)
        return
    }
    header := http.Header{}
    for k, v := range req.Header {
        header[k] = v
    }
    reqTarget.Header = header

    for _, c := range req.Cookies() {
        reqTarget.AddCookie(c)
    }

    var logInfoBuffer []byte

    writeRequestLog := WriteRequestLog
    if writeRequestLog {
        logInfoBuffer := bytes.NewBuffer(make([]byte, 0, 1024))
        reqBody, ok := req.Body.(io.ReadSeeker)
        if ok {

            if err != nil {
                logError.Println("Read request body error:", err)
            }
            logInfoBuffer.Write(bodyBytes)

            reqBody.Seek(0, 0)
        }
    }


    userAgent := req.Header.Get("User-Agent")
    // 如果是代理自己的User-Agent，则可能产生无限循环了，不再处理，直接返回
    if strings.HasPrefix(userAgent, "ZuobaoHttpAsyncAgent") {
        w.WriteHeader(http.StatusInternalServerError)
        w.Write([]byte(`{error: 500, message:"错误，侦测出可能出现无限循环"}`))

        logError.Println("%s 侦测可能出现http无限循环, 忽略请求不作处理. ", reqTarget.URL.String())
        if writeRequestLog {
            logRequest.Println("%s 侦测可能出现http无限循环, 忽略请求不作处理. \n%s", reqTarget.URL.String(), string(logInfoBuffer))
        }

        return
    }





	// 添加自定义的防止无限循环请求的头
	req.Header.Set("User-Agent", "ZbHttpAsyncAgent")


    go func () {

        resp, err := http.DefaultClient.Do(reqTarget)
        if err != nil {
            logError.Println("%s error: %v", reqTarget.URL.String(), err)
            return
        }

        respBytes, err := ioutil.ReadAll(resp.Body)
        if err != nil {
            logError.Println("%s error: %v", reqTarget.URL.String(), err)
            return
        }

        if writeRequestLog {
            logRequest.Printf("%s %s %s => %s", req.Method, req.URL.String(), string(bodyBytes), string(respBytes))
        }


    }()
}



func main() {

	log.SetFlags(log.LstdFlags | log.Lshortfile)

    var err error
    var logRequestFile *os.File
    var logErrorFile *os.File
    var logRequestFilepath = "request.log"
    var logErrorFilepath = "error.log"


    logRequestFile, err = os.OpenFile(logRequestFilepath, os.O_RDWR|os.O_CREATE |os.O_APPEND, 0666)
    if err != nil {
        log.Fatalf("Open log file %s failed: %v", logRequestFilepath, err)
    }
    logErrorFile, err = os.OpenFile(logErrorFilepath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
    if err != nil {
        log.Fatalf("Open log file %s failed: %v", logErrorFilepath, err)
    }

    logRequest = log.New(logRequestFile, "【REQUEST】", log.LstdFlags)
    logError = log.New(logErrorFile, "【ERROR】", log.LstdFlags | log.Lshortfile)


	http.HandleFunc("/", handler)

    listen := ":9090"
    log.Println("started at", listen)
    log.Println(http.ListenAndServe(listen, nil))



}
