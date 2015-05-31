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
    "net/url"
    "os"
    "regexp"
    "strings"
    "time"
)

const USER_AGENT = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/43.0.2357.81 Safari/537.36"

var jsonp_peeler *regexp.Regexp
var http_client *http.Client

func init() {
    jsonp_peeler = regexp.MustCompile(`[^(]\((.*)\)`)
    http_client = &http.Client{}
}

func main() {
    webpage_url, err := get_webpage_url()
    if err != nil {
        print_help()
        os.Exit(1)
    }

    chatroom_id, err := get_chatroom_id(webpage_url)
    if err != nil { log.Fatalln(err) }

    user_key := get_user_key()

    chat_server, chat_channel, err := get_chatroom_url(chatroom_id, user_key)
    if err != nil { log.Fatalln(err) }

    log.Fatalf("server = %#v, channel = %#v\n", chat_server, chat_channel)
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

func get_chatroom_id(webpage_url string) (chatroom_id string, err error) {
    req, err := http.NewRequest("GET", webpage_url, nil)
    if err != nil { return }
    req.Header.Set("User-Agent", USER_AGENT)

    resp, err := http_client.Do(req)
    if err != nil { return }
    defer resp.Body.Close()

    body, err := ioutil.ReadAll(resp.Body)
    if err != nil { return }

    re := regexp.MustCompile(`chatRoomId:'(\d+)'`)
    if err != nil { return }

    match := re.FindSubmatch(body)
    if len(match) < 2 {
        err = errors.New("Can not extract the value chatRoomId from the web page")
        return
    }

    chatroom_id = string(match[1])
    return
}

func get_user_key() string {
    const a = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
    user_key_bytes := [10]byte{}
    for i := range user_key_bytes {
        user_key_bytes[i] = a[rand.Intn(len(a))]
    }
    return string(user_key_bytes[:])
}

func parse_jsonp(jsonp []byte, res interface{}) (err error) {
    match := jsonp_peeler.FindSubmatch(jsonp)
    if len(match) < 2 {
        err = errors.New("Can not parse JSONP data")
        return
    }
    err = json.Unmarshal(match[1], res)
    return
}

var bayeux_reqid int64 = 0

func get_bayeux_reqid() int64 {
    bayeux_reqid++
    return bayeux_reqid
}

func send_bayeux_message(server string, req map[string]interface{}) (res map[string]interface{}, err error) {
    reqid := get_bayeux_reqid()
    req["id"] = reqid
    req_json, err := json.Marshal([1]map[string]interface{}{req})
    if err != nil { return }
    req_url := fmt.Sprintf("%s?message=%s&jsonp=parent.org.cometd.script._callback%d", server, url.QueryEscape(string(req_json)), reqid)

    http_req, err := http.NewRequest("GET", req_url, nil)
    if err != nil { return }
    http_req.Header.Set("User-Agent", USER_AGENT)

    resp, err := http_client.Do(http_req)
    if err != nil { return }
    defer resp.Body.Close()

    body, err := ioutil.ReadAll(resp.Body)
    if err != nil { return }

    var res_tmp []map[string]interface{}
    err = parse_jsonp(body, &res_tmp)
    if err != nil { return }
    if(len(res_tmp) != 0) {
        res = res_tmp[0]
    } else {
        err = errors.New("Bayeux server returned invalid response")
        return
    }
    return
}

var _ = send_bayeux_message

func get_chatroom_url(chatroom_id string, user_key string) (server string, channel string, err error) {
    req_nonce := time.Now().Unix()
    req_url := fmt.Sprintf("http://nas.uc.sina.com.cn/webroom/?type=finance&callback=jsonp%d&roomid=%s&ukey=%s&_=%d", req_nonce, url.QueryEscape(chatroom_id), url.QueryEscape(user_key), req_nonce+1)
    req, err := http.NewRequest("GET", req_url, nil)
    if err != nil { return }
    req.Header.Set("User-Agent", USER_AGENT)

    resp, err := http_client.Do(req)
    if err != nil { return }
    defer resp.Body.Close()

    body, err := ioutil.ReadAll(resp.Body)
    if err != nil { return }

    var json_msg map[string]interface{}
    err = parse_jsonp(body, &json_msg)
    if err != nil { return }

    var ok bool
    if server, ok = json_msg["server"].(string); !ok {
        err = errors.New("Sina did not return a valid chat server")
        return
    }
    if channel, ok = json_msg["channel"].(string); !ok {
        err = errors.New("Sina did not return a valid chat channel")
        return
    }
    return
}
