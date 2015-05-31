/*
  Copyright (c) 2015 StarBrilliant <m13253@hotmail.com>

  Permission is hereby granted, free of charge, to any person obtaining a copy
  of this software and associated documentation files (the "Software"), to deal
  in the Software without restriction, including without limitation the rights
  to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
  copies of the Software, and to permit persons to whom the Software is
  furnished to do so, subject to the following conditions:

  The above copyright notice and this permission notice shall be included in
  all copies or substantial portions of the Software.

  THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
  IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
  FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
  AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
  LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
  OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
  THE SOFTWARE.
*/

package main

import (
    "encoding/json"
    "errors"
    "fmt"
    "io/ioutil"
    "log"
    "math/rand"
    "net/http"
    "os"
    "regexp"
    "strings"
)

const USER_AGENT = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/43.0.2357.81 Safari/537.36"

var http_client *http.Client

func init() {
    http_client = &http.Client{}
}

func main() {
    webpage_url, err := get_webpage_url()
    if err != nil {
        print_help()
        os.Exit(1)
    }

    tvid, err := get_tvid(webpage_url)
    if err != nil {
        log.Fatalln(err)
    }

    flv_url, err := get_flv_url(tvid)
    if err != nil {
        log.Fatalln(err)
    }
    for _, i := range flv_url {
        fmt.Println(i)
    }
}

func print_help() {
    fmt.Fprintf(os.Stderr, "Usage: %s sina-live-url\n", os.Args[0])
}

func get_webpage_url() (res string, err error) {
    if len(os.Args) < 2 {
        err = errors.New("Invalid argument")
        return
    }

    res = os.Args[1]
    if res == "--help" {
        print_help()
        os.Exit(0)
    }
    if !strings.HasPrefix(res, "http://kan.sina.com.cn/u/") &&
        !strings.HasPrefix(res, "http://www.kanyouxi.com/u/") {
        log.Printf("Warning: \"%s\" seems not a valid Sina Live URL.\n", res)
    }
    return
}

func get_tvid(webpage_url string) (tvid string, err error) {
    req, err := http.NewRequest("GET", webpage_url, nil)
    if err != nil { return }
    req.Header.Set("User-Agent", USER_AGENT)

    resp, err := http_client.Do(req)
    if err != nil { return }
    defer resp.Body.Close()

    body, err := ioutil.ReadAll(resp.Body)
    if err != nil { return }

    re := regexp.MustCompile(`tvid=(\d+)`)
    if err != nil { return }

    match := re.FindSubmatch(body)
    if len(match) < 2 {
        err = errors.New("Can not extract the value TVID from the web page")
        return
    }

    tvid = string(match[1])
    return
}

func get_flv_url(tvid string) (res []string, err error) {
    req_url := fmt.Sprintf("http://kan.sina.com.cn/api/kan_2013_getinfo/tvid/%s?random=%.14f", tvid, rand.Float64());
    req, err := http.NewRequest("GET", req_url, nil)
    if err != nil { return }
    req.Header.Set("User-Agent", USER_AGENT)

    resp, err := http_client.Do(req)
    if err != nil { return }
    defer resp.Body.Close()

    body, err := ioutil.ReadAll(resp.Body)
    if err != nil { return }

    var json_msg map[string]interface{}
    err = json.Unmarshal(body, &json_msg)
    if err != nil { return }

    if json_msg["result"].(string) != "succ" {
        err = errors.New("Sina reported error when fetching broadcast information")
        return
    }
    if val, ok := json_msg["stream_url"].(string); ok {
        res = strings.Split(val, ",")
    } else {
        err = errors.New("Sina did not return a valid stream URL")
        return
    }
    for idx := range res {
        res[idx] = strings.SplitN(res[idx], "|", 2)[0]
    }
    return
}
