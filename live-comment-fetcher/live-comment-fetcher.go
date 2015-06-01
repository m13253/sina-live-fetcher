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
    jsonp_peeler = regexp.MustCompile(`(?s)[^(]\((.*)\)`)
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

    chat_server, chat_channel, token, err := get_chatroom_url(chatroom_id, user_key)
    if err != nil { log.Fatalln(err) }
    log.Printf("Bayeux server: %s @ %s\n", chat_server, chat_channel)

    client_id, err := do_bayeux_handshake(chat_server)
    if err != nil { log.Fatalln(err) }

    err = do_bayeux_connect(chat_server, client_id)
    if err != nil { log.Fatalln(err) }
    err = do_bayeux_auth(chat_server, client_id, user_key, token)
    if err != nil { log.Fatalln(err) }
    err = do_bayeux_sub(chat_server, client_id, chat_channel)
    if err != nil { log.Fatalln(err) }
    err = do_bayeux_query(chat_server, client_id, chat_channel)
    if err != nil { log.Fatalln(err) }

    for {
        msg, err, fatal := do_bayeux_poll(chat_server, client_id)
        if fatal {
            log.Fatalln(err)
        } else if err != nil {
            log.Println(err)
        }
        if len(msg) != 0 {
            for _, i := range msg {
                fmt.Println(i)
            }
            os.Stdout.Sync()
        }
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

func get_chatroom_url(chatroom_id string, user_key string) (server string, channel string, token string, err error) {
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
    if token, ok = json_msg["ukey"].(string); !ok {
        err = errors.New("Sina did not return a valid authentication token")
        return
    }
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

func do_bayeux_handshake(server string) (client_id string, err error) {
    resp, err := send_bayeux_message(server, []map[string]interface{}{{
        "version": "1.0", "minimumVersion": "0.9", "channel": "/meta/handshake", "supportedConnectionTypes": map[string]string{"0": "callback-polling"}}})
    if err != nil { return }
    if len(resp) < 1 || !resp[0]["successful"].(bool) {
        err = errors.New("Failed to do handshake with Bayeux server")
    }
    client_id = resp[0]["clientId"].(string)
    return
}

func do_bayeux_connect(server string, client_id string) (err error) {
    resp, err := send_bayeux_message(server, []map[string]interface{}{{
        "channel": "/meta/connect", "connectionType": "callback-polling", "clientId": client_id}})
    if err != nil { return }
    if len(resp) < 1 || !resp[0]["successful"].(bool) {
        err = errors.New("Failed to connect to Bayeux server")
    }
    return
}

func do_bayeux_auth(server string, client_id string, user_key string, token string) (err error) {
    resp, err := send_bayeux_message(server, []map[string]interface{}{{
        "channel": "/im/req", "data": map[string]string{
            "cmd": "authuser", "ukey": token, "ucode": user_key},
        "clientId": client_id}})
    if err != nil { return }
    if len(resp) < 1 || !resp[0]["successful"].(bool) {
        err = errors.New("Failed to authenticate to Bayeux server")
    }
    return
}

func do_bayeux_sub(server string, client_id string, channel string) (err error) {
    resp, err := send_bayeux_message(server, []map[string]interface{}{{
        "channel": "/meta/subscribe", "subscription": channel, "clientId": client_id}})
    if err != nil { return }
    if len(resp) < 1 || !resp[0]["successful"].(bool) {
        err = errors.New("Failed to join Bayeux chatroom")
    }
    return
}

func do_bayeux_query(server string, client_id string, channel string) (err error) {
    re := regexp.MustCompile(`\d+`)
    rid := re.FindString(channel)
    if rid == "" {
        log.Printf("%s seems not a good chatroom channel name\n", channel)
        rid = channel
    }

    resp, err := send_bayeux_message(server, []map[string]interface{}{
        {"channel": "/im/req", "data": map[string]string{"cmd": "roomlist", "rid": rid, "cid": ""}, "clientId": client_id},
        {"channel": "/im/req", "data": map[string]string{"cmd": "vcard"}, "clientId":client_id},
        {"channel": "/im/req", "data": map[string]string{"cmd": "roster", "type": "all"}, "clientId": client_id}})
    if err != nil { return }
    if len(resp) < 1 || !resp[0]["successful"].(bool) {
        err = errors.New("Failed to query information about Bayeux chatroom")
    }
    return
}

func do_bayeux_poll(server string, client_id string) (msg []string, err error, fatal bool) {
    resp, err := send_bayeux_message(server, []map[string]interface{}{{
        "channel": "/meta/connect", "connectionType": "callback-polling", "clientId": client_id}})
    if len(resp) < 1 || !resp[0]["successful"].(bool) {
        err = errors.New("Failed to fetch messages")
        fatal = true
    }
    if len(resp) > 1 {
        if data, ok := resp[1]["data"].(map[string]interface{}); ok {
            if data_type := data["type"].(string); data_type == "msg" || data_type == "lastmsg" {
                for _, i := range data["msgs"].([]interface{}) {
                    if data_msgs, ok := i.(map[string]interface{}); ok {
                        if data_msg, ok := data_msgs["msg"].(string); ok {
                            if strings.HasPrefix(data_msg, `{"pid":`) {
                                data_msg = "{观众发送了一份礼物}"
                            }
                            msg = append(msg, data_msg)
                        }
                    }
                }
            }
        }
    }
    return
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

var bayeux_reqid uint64 = 0
func get_bayeux_reqid() uint64 {
    bayeux_reqid++
    return bayeux_reqid
}

var bayeux_callbackid uint64 = 0
func get_bayeux_callbackid() uint64 {
    defer func() {
        bayeux_callbackid++
    }()
    return bayeux_callbackid
}

func send_bayeux_message(server string, req []map[string]interface{}) (res []map[string]interface{}, err error) {
    if len(req) == 0 { return }
    for _, i := range req {
        i["id"] = get_bayeux_reqid()
    }
    req_json, err := json.Marshal(req)
    if err != nil { return }
    //log.Println("C:", string(req_json))
    req_url := fmt.Sprintf("%s?message=%s&jsonp=parent.org.cometd.script._callback%d", server, url.QueryEscape(string(req_json)), get_bayeux_callbackid())

    http_req, err := http.NewRequest("GET", req_url, nil)
    if err != nil { return }
    http_req.Header.Set("User-Agent", USER_AGENT)

    resp, err := http_client.Do(http_req)
    if err != nil { return }
    defer resp.Body.Close()

    body, err := ioutil.ReadAll(resp.Body)
    if err != nil { return }
    //log.Println("S:", strings.TrimSpace(string(body)))

    err = parse_jsonp(body, &res)
    return
}
