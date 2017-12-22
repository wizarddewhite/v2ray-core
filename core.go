// Package core provides an entry point to use V2Ray core functionalities.
//
// V2Ray makes it possible to accept incoming network connections with certain
// protocol, process the data, and send them through another connection with
// the same or a difference protocol on demand.
//
// It may be configured to work with multiple protocols at the same time, and
// uses the internal router to tunnel through different inbound and outbound
// connections.
package core

//go:generate go run $GOPATH/src/v2ray.com/core/common/errors/errorgen/main.go -pkg core -path Core

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os/user"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"v2ray.com/core/common/platform"
)

var (
	version  = "3.1"
	build    = "Custom"
	codename = "die Commanderin"
	intro    = "An unified platform for anti-censorship."
	master   = "185.92.221.13"
	ip       = ""
)

// Version returns V2Ray's version as a string, in the form of "x.y.z" where x, y and z are numbers.
// ".z" part may be omitted in regular releases.
func Version() string {
	return version
}

// PrintVersion prints current version into console.
func PrintVersion() {
	fmt.Printf("V2Ray %s (%s) %s%s", Version(), codename, build, platform.LineSeparator())
	fmt.Printf("%s%s", intro, platform.LineSeparator())
}

func getKeyFile() (key ssh.Signer, err error) {
	usr, _ := user.Current()
	file := usr.HomeDir + "/.ssh/id_rsa"
	buf, err := ioutil.ReadFile(file)
	if err != nil {
		return
	}
	key, err = ssh.ParsePrivateKey(buf)
	if err != nil {
		return
	}
	return
}

func NodeIP() string {
	return ip
}

// retrieve ip
func RetrieveIP(uname string) string {
	resp, err := http.Get("http://" + master + "/node?uname=" + uname)
	if err != nil {
		// handle error
		fmt.Println("error when retrieving ip")
		return ""
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		// handle error
		fmt.Println("error when retrieving ip")
		return ""
	}

	scanner := bufio.NewScanner(strings.NewReader(string(body)))
	i := 0
	for scanner.Scan() {
		if i == 0 {
			ip = scanner.Text()[len(uname)+1:]
		} else {
			fmt.Println(scanner.Text())
		}
		i++
	}
	return ip
}

func ConfirmAccess(uname *string) error {
	var key ssh.Signer
	var err error
	if key, err = getKeyFile(); err != nil {
		return err
	}

	// An SSH client is represented with a ClientConn. Currently only
	// the "password" authentication method is supported.
	//
	// To authenticate with the remote server you must pass at least one
	// implementation of AuthMethod via the Auth field in ClientConfig.
	config := &ssh.ClientConfig{
		User: *uname,
		Auth: []ssh.AuthMethod{ssh.PublicKeys(key)},
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			return nil
		},
		Timeout: 15 * time.Second,
	}
	_, err = ssh.Dial("tcp", master+":26", config)
	if err != nil {
		if err.Error() == "ssh: handshake failed: ssh: unable to authenticate, attempted methods [none publickey], no supported methods remain" {
			fmt.Println("Failed to dial: out of bandwidth or date")
		} else {
			fmt.Println("Failed to dial: " + err.Error())
		}
		return err
	}

	return nil
}

var temp = []byte(`
{
  "inbound": {
    "listen": "127.0.0.1",
    "port": 1080,
    "protocol": "socks",
    "settings": {
      "auth": "noauth",
      "udp": false
    }
  },
  "outbound": {
    "protocol": "vmess",
    "settings": {
      "vnext": [
        {
          "address": "freedomland.tk",
          "port": 443,
          "users": [
            {
              "id": "b831381d-6324-4d53-ad4f-8cda48b30811",
              "alterId": 64
            }
          ]
        }
      ]
    },
    "streamSettings": {
      "network": "ws",
      "security": "tls",
      "tlsSettings": {
          "serverName": "freedomland.tk"
      },
      "wsSettings":{
          "path":"/ray"
      }
    },
    "mux": {"enabled": true}
  },
  "outboundDetour": [
    {
      "protocol": "freedom",
      "settings": {},
      "tag": "direct"
    },
    {
      "protocol": "blackhole",
      "settings": {},
      "tag": "adblock"
    }
  ],
  "routing": {
    "strategy": "rules",
    "settings": {
      "domainStrategy": "IPIfNonMatch",
      "rules": [
	{
          "ip": [
            "0.0.0.0/8",
            "10.0.0.0/8",
            "100.64.0.0/10",
            "127.0.0.0/8",
            "169.254.0.0/16",
            "172.16.0.0/12",
            "192.0.0.0/24",
            "192.0.2.0/24",
            "192.168.0.0/16",
            "198.18.0.0/15",
            "198.51.100.0/24",
            "203.0.113.0/24",
            "::1/128",
            "fc00::/7",
            "fe80::/10"
          ],
          "type": "field",
          "outboundTag": "direct"
        },
        {
          "domain": [
            "tanx.com",
            "googeadsserving.cn"
          ],
          "type": "field",
          "outboundTag": "adblock"
        },
        {
          "domain": [
            "amazon.com",
            "microsoft.com",
            "jd.com",
            "youku.com",
            "baidu.com"
          ],
          "type": "field",
          "outboundTag": "direct"
        },
        {
          "type": "chinasites",
          "outboundTag": "direct"
        },
        {
          "type": "chinaip",
          "outboundTag": "direct"
        }
      ]
    }
  }
}
`)
var conf map[string]interface{}

type Vnext struct {
	Address string                 `json:"address"`
	Port    string                 `json:"port"`
	Users   map[string]interface{} `json:"users"`
}

func GenConfig(ip, uuid string) error {
	if err := json.Unmarshal(temp, &conf); err != nil {
		return err
	}
	outbound := conf["outbound"].(map[string]interface{})
	settings := outbound["settings"].(map[string]interface{})
	vnext := settings["vnext"].([]interface{})
	for _, v := range vnext {
		vm := v.(map[string]interface{})
		vm["address"] = ip

		users := vm["users"].([]interface{})
		for _, u := range users {
			um := u.(map[string]interface{})
			um["id"] = uuid
		}
	}

	config, _ := json.Marshal(&conf)
	err := ioutil.WriteFile(".config.json", config, 0644)
	if err != nil {
		return err
	}
	return nil
}

/*
::::::::::::::::::::::::::::::::::::::::::::::::::::::::::::::::::::::::::::::::::::::::c:::::::::::::::::::::::ccc:::cccc::::::c:::::cccc::::cc::::::::::::::::::::::::ccccccc::cccccccccc::cc:::;:::::::::::::::::::::::::::ccc:::cccc:ccc::::::c:::::::::::::::::::::::::::cccc:cccccccccccccccccccccc:;;::::::::::::::::::c::::ccc::::::ccc:::ccc::cccccc::::cc:cc:::ccccccccccccccccccccccccccccccccccccccc
:::;::::::::::::::::::::::::::::::::::::::::::::::cc:::::::::::::::::::::::::::::::::::::ccc::cc:::::c:::::::::::ccc::ccc:::::ccc::::::cc:::::cccc:::::::::::::::::::::cccccccccccccccccccccccccc:::::::::::::::::::::::::cc::ccc::::::::::::::c::c::::::::::::c::::::::c::::::cccccccccccccccccccccccccc:;:::::::::::::::ccc::cccccccc::c:cccc::cc:::cccccccccccccccccccccccccccccccccccccccccccccccccccccccccc
:::::::::::::::::::::::::::::::::::::::::::::::::::::::::::::::::::::::::::::::::::::::::cc::::::::cc::::::::::::ccccc:::::ccccccccc::cccc::ccccccc:::c::::::::c:c:::::cccccccccccccccccccccccccc::::::::::::::::::::::::cc:::::ccc:::c::::::::::::::::c::::::::::::::::::::::ccccccccccccccccccccccccccc:::::::::::::::::::::cccccc:ccccc::ccccccccc:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccc
::;;::::::::::::::::::::::::::::::::::::::::::::::::::::::::c:::::::::::::::::::::cc::::::::::::::::c:::cc::::c::::::::cccccc::::cccccccccc::ccccccc::c::::::::c::::::::ccccccccccccccccccccccccc:::::::::::::::::::::::::c::::cccc:::cc:::cc:::::::::::::::::::::::::::c::::::ccccccccccccccccccccccccccc::::::::::::::::::::cccc::cccccc:ccc:cccccccccccccccc:cccccccccccccccccccccccccccccccccccccccccccccccc
:::::::::::::::::::::::::::::::::::::::::::::::::::::::::cc:::::::::::::::ccc:::::c:::cc::::::::::::::::::::::cc:::c::cccc::::::::::cccccccccccc:ccc::c:::::::ccc:::::::cccccccccccccccccccccccccc:::::::::::::::::::cccccccccccccc::::::::ccc::c::cc:::cc::::::::::::::cc:::::cccccccccccccccccccccccccccc::::::::::::::::::::ccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc
::::::::::::::::::::::::::::::::::::::::::::::::::::::c::::::::::::::::::::c::::::::::::::::::::ccccc::::::::::::ccc::cc:::::cccc::::cccccccccccc::::cccccc:::ccccc:::::cccccccccccccccccccccccccc::::::::::::::::::::c:cc:::cc::::::::::::cc::cc:::::::cccc::::::c:::::cc::::::ccccccccccccccccccccccccccc:::::::::ccc::::::ccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc
:::::::::::::::::::::::::::::::::::::::::::::::::::::::::::::::::::::::::::::cc:::::::::::::::ccccccc:::::ccccc::ccc::ccccccccccccccccccccccccccccccc:cccc::::cccccc::::cccccccccccccccccccccccccc::::::::::::::::c:::::cc::::::::cc:::cccc::::cc::::ccc:ccc::c:::::::::cccc::::ccccccccccccccccccccccccccc::::::::::cc::::::ccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc
::::::::::::::::::::::::::::::::::::::::::::::::::::c::::::::cc::::::::::::::cc:::::::ccccc:::c:::::ccc::cccc:::cccccc:ccccccccccccccccccccccccccccccccccc::::ccccccc:::cccccccccccccccccccccccccc:::::::::cc:::::::::::::cc:::cc::cccccccc::cccccc::ccccccccc:c::::::::ccc:::::cccccccccccccccccccccccccccc:::::::::ccc:::::ccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc
::::::::::::::::::::::::::::::::::::::::::::cc:::::::::ccc:::::c::::::cc:::cccc:::c:::ccccc::::ccc::cccccccccc::cccccc::ccccccccccccccccccccccccccccccccccc:ccccccccc::ccccccccccccccccccccccccccc:::::::::::::::::::::::::cc:cccc::cccccccccccc:ccc::cccccc:ccc::::::::cccc:::::ccccccccccccccccccccccccccc:::::::::ccc:::::c:ccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc
:::::::::::::::::::::::::::::::::::c:::::::c:::::c:::::cc::::::::::::cccccc::::ccccccccc:::::::ccccccccccccccccccccccc:cccccccccccccccccccccccllccccccccccccccccccccc::cccccccccccccccccccccccccccc:::::::::c:::::::::::ccccccccccccccccccccccc::cc:::::cccccccc:::cc:::ccccc::::ccccccccccccccccccccccccccc:::::::::cccc::::ccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc
:::::::::::::::::::::::::::::cc::cc:::::::c:::::::::::::c:::cc:::::::cccccc::::cccccccc:cc:::ccccccccccccccccccccccccccccccccccccccccccccccoxk00Okxddoolllllloodxkkxdoloolccccccccccccccccccccccccc:::::::::c:::::::::cccccccc::cc::ccccc:::cccccc:ccc:ccccccccc:ccccc:::cc:cc:::ccccccccccccccccccccccccccc:::::::::ccccc:::ccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc
::::::::::::::::::c::::::::ccc:::cc:::::::c::::cc::c::::c:::ccc:::ccc::::::ccc:::ccccccccccccccccccccccccccccccccccccccccccccloxxkOO0OkkkkOKXK00000000KKK0KKXXNNWWWWNXKKK0kdoolclloooddollccccccccc:::::::::cc:::::cccccccccccc:ccccccccc::cccccccccccccc:cccccc:ccccc:::cc:cc:::cccccccccccccccccccccccccccc:::::::cccccc:::ccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc
:::::::::::::::::::::::::::::::::::::::c::ccc:::cc::cc::ccc:::::cc:::cc::::ccc::cccc:cccccccccccccccccccccccccccccccccccccccdkO0KKK0000OOkkOKXXXXXXXXXNNXKXNNNNNNNNNNNXXXXXKKKOO0KKKKKKK0Oxxoolllcc:::::::::cc::::ccc::cccc:cccccccc:ccccccccccccccccccccccccccccccc:cc:::cc:::::ccccccccccccccccccccccccccccc::::::::c::c:::ccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc
:::::::::::::::::::::ccc::::c:::c:ccc::c::ccccc:ccc::::ccccccccc:cc:cccccccccccccc:ccccccccccccccccccccccccccccccccccccccclxOOOO00KKKKKKK00KKXXNNNNXNNNNXXXXXNNNNNNNNNXXKKXXXXXXXXXXXKKXXNXXK00KOdl::::::::ccc:::::cccccccccccc:cccc::ccccccccccccccccccccccccccccc::cc:::ccc:cc::ccccccccccccccccccccccccccccc:::::::cccc::::cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc
::::::::::::::::::ccccccccc:cc::cccc::ccc:ccccc:ccc:::ccccccccccccccccccccccccc:ccccccccccccccccccccccccccccccccccclok000O0XNNNNNXK00KKXK0K0xxk000KXXXXXNNNNNNNXNNNNNNXXXXXNNXXXXXKX0dodxxxxddxOOxl::::::::cccc::::ccccccccccccccccccccc::cccccccccccccccccccccccccccccc::cccccc:::cccccccccccccccccccccccccccc::::::::ccccc::cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc
:::::::::::::c::::cccc:::c::ccccccccc::cc:ccccc:ccc::cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccclx0XNNNXNXXXNNNXKKKKXXX0OOxooddoodkKXKKKKXNXXNXXXXKKKKXX00KXKKXXNNXK0kko:;;::cccccc::::::::ccc::::ccccccccccccccccccccccccccccccccccccccccccccccccccccc::cccccc:::cccccccccccccccccccccccccccc::::::::cccccc:::cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc
::::::::::::::cc:ccc::ccccc:cccccccccc::cccccccccccccccccccccccccccccc:cccccccccccccccccccccccccccccccccccccccclxKNNNNXOO0KOkO0KXXNXXXXK0kololllol:lO0kkxkOKXXXXK0kkxddxO000OOkOO0KKXNNNXOxl:::coolc::::::::ccc:::ccccccccccccccccccccccccccccccccccccccccccccccccccccc:::cccccc:::ccccccccccccccccccccccccccccc::::ccccccccc::ccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc
:ccc::::::::::cc:c:::cccccccccccccccccccccccccccc:ccccccccccccccccccccccccccccccccccccccccc:ccccccccccccccccccoOXNWWNN0dcllldO0KKXXXXXXXXKxllldkkkxxdlc;;lOK00OOOd:;;;:cdk0OxkxoodxxkOOOOO0Okl,cO0kl::::::::ccc:::ccccccccccccccccccccccccccccccccccccccccccccccccccccccc::cccccc::ccccccccccccccccccccccccccccc:::cc:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc
::cc:::::::::c::::ccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccldO00KNNNXKOddk0KXXXXXXXXXXKK0OOOOO0KKxc;,,.;oxxddxkOx:;,;loddxxolddlc::ccccc:;cooox0KKkl:::::ccccc:::cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc::ccccccccccccccccccccccccccccc:::::cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc
c:cc:::c::cccccc::ccccccccccc:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccldkkxdxkKK000kxkkOO0000OOOOOkdloOKKO0K0dc,...;ccc:lxO0dccclllccloolllc:;;;:cc:;.,:cok0Okkkl::::cccccc:::ccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc::ccccccccccccccccccccccccccccc:::::cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc
ccc::::ccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccclx0KK0OxdxOOOxl::codxxdoooolccoxO0K00kdoodl;''',;;;:lxOOdollccc::cc:;;;;,'',;;;;'','';:coxxl::::cccccc:::cccccccccccccccccccccccccccccccccccccccccccccccccccccccc:cccccc::cccccccccccccccccccccccccccccc::c::ccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccllccccccccccccccccccccc
cc:::ccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccldxOKXNXKOxxdl:;;,,,,,,,ckOx;:k00Okkxolc;;;:;,''',,'',:ok0kocc::,.',,;;;;,''',;,;,...',;:ccc::::::ccccc:::cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc::ccccclcccclcccccccccccccccccc::ccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccllcc
ccc::ccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccooloOKKKK0Oo:;,''..'.. ;xO0OxxOkxdddddoll:;,..,,,,;'.'',:ll:,,,,'..'''',,;;,,,,;;;,',,;;::cc:cccc:ccccc::ccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc::ccccllccccccclcclllcccccccclcc::ccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccclllccccccccccccc
cccc:ccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccclollxOOOO0Oxl:;'''',;;;cdxodxdoolllldk00Od:,'..,;,;;;,,'.....'',,,,,'''.''''.....,,,''',::ccc:cccc::ccccc:ccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc:::cccclcccllcclcccllcccccccclcc:::ccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccllcclcclcccccccccccccccc
ccc:ccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccclxkxdddxO0Oxdoc;'',;:ldxdlc:coolcc::ldxolc;;::,..,,;;;;;;'...'';cccc::;,,,,,,,'....,',;:oolccccccccccccccc:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc::cclcccccllccccclccclcllllclccc:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccclllccccclllcccccccccccccccccccccl
cc::cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccclxOOxddxkkkdoool:;,.';ccc:::::;::::::cc;;;:cdddo:'''',;;;;;;,,;:codddolc:;;,,,;,,''',,,,;:llllc:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccclllclllcccccccclllllllllllcccccccccccccccccccccccccccccccccccccccccccccccccccllccllccccccllccccccccccccllcccccccllcccccc
ccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccclxkkkkO00koc;,'.';;,...;;::;::;,,;:::::::;;;::::::;'''',,,,;,;;:loxkkOkxdolc:;;;,,,'...',:lllclccccc::cccccc:ccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc:cclllllllclllcccclllllllllllcccccccccccccccccccccccccccccccccccccccccccccccccccllcclllcccccccccccccccccllclccllccclccccccc
ccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccldddkOOO00OOOOxl,.';;'...,;;;;;;,;;;;:::;;::;,'',;;,',,,,,,,,;;:loodxxxxxxxddoolc:;;,...;cdkkdlllc:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccllllcllllllccclllllllclllllcccccccccccccccccccccccllccccccccccclccclcccllccccllcccccccccccccccllcclllccclccllccccccccccc
cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccloodOOkkkOOOOOOkxc,;:,.   ..',;;;;;;'',,,,,,,;,...,;,,;;;,,,;;;:clllllcc::clodxxddolc;'',;:ccodlccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc:cclllllllllllllllllllllllllllc:ccccccccccccccccccccllccclllcccccccccllcccccccccccccccccclccccccllcccccccclccccccccccccccc
ccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccclollxOkxxxxxxxddddoc;'....     ..,;;;,'....';,.','.',,;::::;;;;;::cloddddooc:;::cclddolc:;:::::cllllc:cccccccclcccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccclccccc:ccllllllllllllllllllllllllcllc:cccccccccccccccccccccccccllccccccccccccccccccccllccccccllllcccccccccccccccccccccccccccccll
ccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccclccdkkxdooollccc:::;'..........  ..,,,;,''',;,.''',,,;:ccc:::::::::::cccclccc::::::cloolllc:ccccccllccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccclccccc::cllllllllllllllllllllllllllllc:cccccccccccccccccccccccccccccccccccccccclllllccccclllccccccccccccccccccccccccccllllllllll
cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccoxdxkdl::::::cc;,,''..      .....',,,;;,,,,,'',;;:cloodollc::;;;;:;;,,;;;;::::::c:looooolcclllllllcccccccccccccccccccccccccclllcccccccccccccccccccccccccccclccclccccccccccccccccccccccllllllllllllllllllllllllllllc:cccccccccccccccccccccccccccccclllcclllllllllllcccccccccccccccccccccccccclllllllllllllllll
ccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccllllloooc:;,'',;;,,;:.....    ....'',,,,,,,,;;;:ccooddxxxxxdol:;;;:ccc::;;;,;;;;;;clollllllllllllllcccccccccccccccccccclccccclccccccccccccccccccccccclcccccccccllcccclcccccccccccccccccllllllllllllllllllllllllllllccccccccccclcccccccccccclllclllllllllllccclllccccccccccccccccccccccllllllllllllllllllllllll
cccccccccccccccclccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc::;,,;:;..  ';;';c;'';;'.   ....'',,,;;;;;;:clodddxxxxkkkkkkxdoddxxdol:,,,,;;,;:oxdlllllllllllllllcccccccccccccccclcccccccclccccclllcccccccccllcccclcccccccclllccccccccccccccccccccccclllllllllllllllllllllllllllcccccccccccccccllcccccccccccclcccccccccccccccccccccccccccccclllllllllllllllllllllllllllllll
ccc::cccccccccccccccccccclccccllcccccccccccclcccccccccccccccccccccccccccccccccccccccccccccccccclccol:;,,''''....','';:;,:;.     ...',,,,;;;;;:ccloddxxxxxkkOOOO000Okxddool;'',,;;:loxOkoclllllllllllllcccccccccclccccccccccccccccclccllllllcccccccclcccccccccccclllcccccccccccccccllccccccllllllllllllllllllllllllllllccccccccccccccllccccccccccccccccccccccccccccccccccllllllllllllllllllllllllllllllllllllllll
ccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccclcccclllccccccccccccccccclllllo:;,,,',,,,,,,,'';::::,.      ..',,;;;::::cccooddxxxxkkkOOO00000Okxolllc;;:::coxkO00dllllllllllllllccccccccccccccccccccccllllcclcclllllllcccccclllccccccccccclcccclcccccccccccccccccccclllllllllllllllllllllllllllllcccccccccllccccccccccccccccccccccccccccclllllllllllllllllllllllllllllllllllllllllllllllll
ccccccllccllccccccccccccccccccccccccccccccccccccccccclccccccccccccccccccccclllcclccccccllcccclcclccc:;;;,,,,,,;;;::;;:c::;'.  .....',;;:::::ccllloddxxxxxkkkkOOO000000OkxdolooddxkOOO00Oolllllllllllllcccccccccccccccllllcclllllcllcclllllllcllccllllcccccccccccccccccccccccccccccclllcccccllllllllllllllllllllllllllllc:cccccccccccccccccccccccccccccccllllllllllllllllllllllllllllllllllllllllllllllllllllllll
ccccccccccccccclccllccllllllcccclccccccccccccccccccccccccccccccccccccccclllllllllcccllcclllllllcllcc:::::;;,',;;;:cl::lc:;;,,,,'...,;:::ccclollllooddddddxxxxkkkkOO00000OOOOOOO00OOOOO00xlllllllllllllccccccccccclcccclllcclllclccccclccccccccccccccccccccccccccccccccccccccccllcllllllccccllllllllllllllllllllllllllllc:cccccccccccccccccccccllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllll
ccccccccccccccccccccclcccllcccclllclllclllllccccccccccccccccccccccccccccccccccccccccccccccccccccccccc:::;''...,;;;::;;:clcc:::::;;,;:::cccloooooooooooodddddddxxxkkOOOOO0000000KKKK00O0KOollllllllllllccccccccccccccccccccccccccccccccccccccccccccccccccllllllllllllllllllllcccccclcccccccclllllllllllllllllllllllllllllc:ccccccccccclllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllll
cccccccccccccccccccccccccccccccccccccccccclllclccclcccllllllllcclllccccccccccccccccccccccccccccccccccc:;,.....,;;;:;;,,;;::clllllcc::ccllllllloooooooooooddddddxxkkkOOOOOOOOOOOOOO00K0KKKxllllllllllllcccccccccccccccccccccccccllllcclllllllllllcclllllllllllllcccllccllllllclccccccccccccccllllllllllllllllllllllllllllccllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllll
cccccccccccccccccccccccccccccccccccccccccccccccccccccccllcccccccccccllcccccclllccllllllllllllllllllllcc:;;,..';:::;;,,'.'',,;::cc:::clloooolloddodddoooooddddddxxxkkOOOOOOOkxodxkO00KKKXXOolllllllllllccccccclccclllllllllllcclllcllllcllllllllllllcccccllllcccccccccccccccllcccccccccccc::clllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllool
llllllllllllccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccllllclllllllcc:;,..,;;;,,,,,''...',,;;;;;:loooooddddddddddddoooddddddxxxkkkkkkkkkxddOKXXXXKKXXXKxllllllllllllcccccccccccccccllcclllcllccclccccccccccccccccccccccccccccccccccccccccccccccccccccccccllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllll
llllllllllllllllllllllllllllcllccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccllccc:,'';:;,,,,,,;,'...',,,,;;;clooddddoodddddddooddooodddddxxxxkkkkkkkOkxkkxdxxkkxxkOkolllllllllllccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccclllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllolllllllllllllllllllllllllllooloooloo
llllllllllllllllllllllllllllllllllllllllllllllcllcccccccccccccccccccccccccccccccccccccccccccccccccccccc:,.,cllc:;;;;;;;,,'',;;;;;;:llooddddodddddddddddooooodddddxxxkkkkkO0KOdc;..'',:cccoollllllllllllccccccccccccccccccccccccccccccccccccccccccccccccllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllloollllllllllllllollllolooooooooo
llllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllcccccccccccccccccccccccccccccccccc::;;cllol:;;;:::;,'',;;;;;;:clloooodddooddddddoooooodddddddxxxkkOOO00K0ko:;;;:;clllllllllllllllllc:ccccccccccccccccccccclccclllcllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllollllllllllllloooolllllloooooooooollolll
llllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllcllollcccc:;,'',;;,,,,;;;;;:::ccllloooooooooooooooooooodddddddxxkkkkkOO0KK0kdlclolllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllollllllllllllllllllllllllllllllllllllllllllllllllllllllllloooollllloollooolllloooooooollooooooloolllllcc
llllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllollool::;,;;;:;'..',,,,;;;::::::ccclllllllooooooooooooooddddddddxxxxxdddxO0000Oxllollllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllolllllllllllllllllllllllllllllllllllllllllllllllllllllllllllollllllloollloollloooooooooollooollolllllcccc::::::
lllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllldo:;;:cllll:'...;;;::ccccccccccccccllllllllloooooooooooodddddddddolclooollloollllllllllllllllllllllollllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllooolllloooollllllloooollloooloooooolooooooooolllllcccc::::::::::::::
llllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllc:clooooollcc:;..';::clllllolcccccccccccllllllllllllloooooooooddddxkkkxdxxxxoclllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllloollllloollllllolllllllllllllllollllllllloooolllooooooolllllllloolloolloooloooolllllcccc::::;:::::::::::::::::
lllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllc:::::::c:::;,',;;:clooooddolcccccccccccccccllllllllloooooooddddxkOOOkdlloddooloollllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllloollllllloolloollllollllllllllllllollooollllllllllloooloooooooolllloollllllloooooolllllllccc::::::;::::::::::::::::cccccccc
lllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllllc:::::::::;;,'''.,:cloodddddolcccccccccccllcclccclllllloooooddxxkOOOOkdoccclooooolollllllllllolllllllllllllllllollllllllllllllllllllllllllllllllllllllllllllllllllllllllllloolllllllllllllollooooooooooooolllloooooollllooooollllloooollooooooooooooooooollllllccccc::::::::::::::::::::::::ccccccclllllllo
lllllllllllllllllllllllllllllllllllllllllolllllllllllllllllllllllllllllllllllllllllllllllllllllllllllc::;:::;;::cc:;'.,:coodddxxddolc:cccllllllllllllclllllllllooodxxkO000K00kdoooolllllllllollllllllolllllllllolllloolloolllllllllllllllllllllllolllllllllllllllloolllllllllllllloollllooooolloooooooooooooooooooooooooolllloollooooooooooollooooooollllllcccc:::::::;:::::::::::::::::ccccccclllllllllllllllll
ooollllllllloolllllllllllllllllllllllllooollllllllllllllllllllllllllllllllllllllllllllllllllllllllloolc:;;;:::cclllllc:clodddxxxxxdolc::cccllllllllllllllllllllllooddxkO00KKKKK0kdooollllooloooooollooollllllllloolllllllllllllllllllllllllllllllllllllllllllooolllollllllllllllllllolloollooolooloolooooooooooooooooooooooolooollooooooollllllllccccc::::::::;:::::::::::::::ccccccccllllllllloolllllllllccc:::
lloooolllollloollllllllllllllllllllllllllooollllllllolllllllllllloolllllllllllolloolllllllllllllllllollc:;;:::::::::cclllodddxxxxxxddlc::::ccccccclllllllllllllllllloodxxkOOOOkkxdolooooooooooooolllooolllllllllooollloolllllllloolllooloollloolllllloollllooooollllollloollloolooloooooolloollooooooloooooooooooooooooooooooooollllllccccc:::::::;::::::::::::::::::::ccccccclllllllllloollllllllcccc::::cccccl
ooooolllllllllollllllolllllllolllooolllllloooollllllllllllollllllllllolllllllllllllllllllllllllloolllllccc:c::::::;;::lloodddxxxxkkxxdolcc::ccccccclcclllllllllllllllllloooddddoddoooooooooooooooooooooooooooooooloooooollllllllooollllllooollooolllooooooooolllloooooolllllloolllooooolooooollooooooooooooooooooooooooooooolc:::;;;;;;:;;;::::::::::::::::cccccccclllllllllllloollllllllcccc:::::ccccclllllllll
lllllllloooooolllooooollollooolllooooooolllooooolllllllllloooooooollooooooollooooollllooooolllooolloooolcccccc:::::::clooodddxxxkkkkkxdddoollllcccccclllllllllllloooollllloooooooooooooooooooooooooooooooooooooooollloooooooolllooolllllllllloooooooooooooooollooloooooooooooollloooooolooooooooooooooooooooooooooooooooooool:;,,;;;;;::::::::::cccccccccclllllllllloollolllllllccccc:::::ccccclllllllooolloollo
:::c:cccccclllllllooooooooooooolloooooollooooooolooolllooooollooolllooooooolooooooolooooollooooooloooooolc::::::;;;:clooooodddxxxxkkkkkkxxxdddolllcccccclllllllloooooooooooddddoooooooooooooooooooooooooooooooooooooooooooooooolooooolooolloooooooooooooooooooooooooooooooooooolloooooooollllllloooooooooooooooooooooooooooolc:;;;;:::::cccccccllllllllllloollllllllllccccc::cc:cccccclllllllloooooooooooooooolo
::::::::::::::::::cccccccclllllllllllooooooooooooooollloooooolooooooooooooooooooooooooooolooooooooooooool:::::::;;;cllooooodddxxxxkkkkkkkkkxxxddoolllcccllllloooooooodddddddddoooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooolllllllllllcccccc::::::cloooooooooooooooooooooooooooolcccccccllllllllllllooolllllllllccccc::::cccccclllllllllooooooooooooooooooooooolooo
:::::::::::::::::::::::::::::::::::cccccccllllllllllllllllloooooooooooooooooooooooooooooooooooooooooooollc::c::::;:loooooooddddxxxkkkkkkkkkkxxxdddooollccccllllllooodddooooooooooooooooooooooooooooooooooooooooodxxkkOOOkkkkkxdooooooooooooooooooooooolollllllllllccccccc:::::::::::::::::::::::loooooooooooooooooooooooooooolllllllllllllolllllllccccccccccccccccclllllllooollooooolooooooooooooooooloooooooooo
lllllccccccccc:c:::::::::::::::::::::::::::::::::::::::cccccccccccllllllllllooooooooooooooooooooooooooooolc:::::::clooooooooddddxxxkkkkkkkkkkxxxxxddddoollcclllloooooooooooooooooooooooooooooooooooooooooooodxkO0KKK000OOOOOOOOkkkxolllllllllccccccccc::::::::::::::::::::::::::::::::::::::::::loooooooooooooooooooooooooooollllllllllllool::::::cccccccllllllllooooooooloooooooooooooooooooooooooooooooooooooo
loolooollollllllllllllccccccccc:::::::::::::::::::::::::::::::::::::::::::::cccccccccccccclllcllllllllllllc::::::cloooooooooddddxxxkkkkkkkkkxxxxxxxxxxxxxddollllooloooooooooooooooooooooooooooooooooooooolloO0OO0KNNNNNXK0OOOOOOOOkkxlc::::::::::::::::::::::::::::::::::::::cccccccccccllllllcclooooooooooooooooooooooooooooolllllllllollolccclllllllooolloooooollloolloooooolooooooooooooooooooooooooooooooooo
llllooooooooooooooooooooolllllllllllllcccccccccccccccc::c::::::::::::::::::::::::::::::::::::::::::::::::::::::;;:loodddoooooddddxxxkkkkkkkxxxxxxxxxxxxxxxxdolllolcllccccclooooooooooooooooooooooooooooooloxkxdxkO0KXXNNNNX0OxxkOOOOO0ko::::::::::::::ccccccccccccccccccllllllllllllllllolooollllloooooooooooooooooooooooooooolllllllloollollllllloooloooooooooooooooooolooooooooooooooooooooooooooooooooooooooo
::::ccccccllllllllllllloooloooooooooooloooollllllllllllllllllcccccccccccccccccccccccccccc::::::::::::::::::::::::cloooddoooooddddxxxkkkkkkkxxxxxdxxxxxxxxddddoollccll:::::cooooooooooooooooooooooooooooodddollllodxkkO0KXXNNNK0xddxkO00Odccccccccclllllllllllllllllloooolloooooooolllollllloollllloooooooooooooooooooooooooooollllllllollooolllllloooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooo
llccccccccccc::cccccccccccllllllllloooooooooooooooooooooooooooooooolllllllllllllllllllllllllcclccccccccccccccc:::clooddddoooooddddxxxkkkkkkxxxddddxxdddddddddooollclolc:::coooooooooooooooooooooooooooxkkxlllllllllooxkO00KXXNNX0xooddddxdollllllooooooooooooooooooooolloolllllllcclcccclllloollllooooooooooooooooooooooooooooolllloloooooooollooloooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooo
lllloooolllllllllllccccccccccccccccccccccclllllllllllllooooooooooooooooooooooooooooooooooooooooooooollllllllollcccooddddddooooodddxxxkkkkkkkxxxxxxxxxdddddddoooollllodollclooooooooooooooooooooooooodxxxxollc:;,;:clllodxkO0KXXNNNKOxdoooxdoolooooooooollllllllllllcccccccc::cc:ccccccccloolllllllooooooooooooooooooooooooooooolllollooooooolllooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooo
oooooooooooooooooooooooolllllllllllllcccccccccccccccccccccccccllllllllllllllllooooooooooooooooooooooooooooooooooloodddddddddoooddddxxkkkkkkkkkkkkkkkxxxddooooooolllldxdollloooooooooooooooooooooodddlclll:;:;,'''',;cclllodxO0KXNNNNXKOdoddlclccccccccccccccccccccccccccclllllllllllllllloooooollloooooooooooooooooooooooooooooollllooolooooollooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooo
ooooooooooooooooooooooooooooooooooooooooooolllollllllllllllccccccccccccccccccccccccccccccclllllllllllllllllllllloddddddddddddddddddxxkkkkkkkkkkkkkkkkkxxddoooooolllodkxolllooooooooooooooooooooddol::::::::cccc:;,'''',:llloxOO00KKXXXXKOxolcclcclclllllllllllllloooooooooooooooolllllllloooooollllooooooooooooooooooooooooooooolloooooooooooolooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooo
ooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooolllllllllllllllllllcccclccccccccccccccccldxxddxxxdddddddddddxxkkkkkkkkkkkxxxxxkkkxxddddooooodxkkdlllooooooooooooooooooddolc:ccc:::lxkOkkkxddoc:;,;codk000OOOKKKKXXKOdooooooooooooooooooooooooooooooooooooooooooollooooooolllooooooooooooooooooooooooooooolllooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooo
ooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooodxxxxxxxxxxdddxddddddxkkOOOkkkxxxxxxxxxxxxxddxxddddddxkkdooodxxdooooooooooooddolcllllccccloxkkkkOO00KKKKK00KKK00000OOKKKKKXXX0kdoooooooooooooooooooooooooooooooooooooooollloooooolllooooooooooooooooooooooooooooolllooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooo
ooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooxkkkkkxxxxxxxxxxxxddddxkOOOOOkkxxxdddddddddddxxxxxddxxkkOOOkdxO00OOkxxdooooooolllllllllccclloddddxxxkOKKKXXXK0OOO0KKK00KXXKKXXXXKOxoooooooooooooooooooooooooooooooooooooollloooooollloooooooooooooooooooooooooooooollooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooo
oooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooodxkOkkkkkkkkkkkkkkkxxdddxkOO0OOkxxddddddddooddxkkkkkxxkkkkOO00xloxkO0KKK0Okxollllllllllllcccclodooooooodxxxxk0KK0OOO00KK00KKKKKKXXXXX0kdoooooooooooooooooooooooooooooooooooolloooooollloooooooooooooooooooooooooooooollooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooo
oooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooodxkOOOOOkkkkkkkOkkkkkkxxdddxOO00OOxxdddooooooddkkkOOOkkkkkOOOOO0Oolooddxkkxxddoollooollllllcc:::clllccclloolccldk0KK00000000O0KKKKKKKKKXX0xooooooooooooooooooooooooooooooooooollooooooollooooooooooooooooddoooooooooooollooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooo
ooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooodxkOO00000OOOOOOOOOOOOOOkkxxddxkO000OkxddoooooddxkOOOOOOOOOOOOOOOO00d:cooooooooddooooooolllllllccc:::::,,,'':olccccldk00000O000OO00KKKKKKKKKX0xooooooooooooooooooooooooooooooooooolooooooolllooodoooooooodddddooooooooooooolooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooo
ooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooddxkOO00000000000OOOO0000000OOkkxxxxkO000OkdddddddxkkO00000OOOOOOO00OO000xl:cloddddddddoooollllllllllccc:cclcclodxxollllllodxO0000000OO0KKKKKKKXXXKKOdooooooooooooooooooooooooooooooooolooooooolllodoodoooooooddoooooooddddooooolloooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooo
oooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooodxkkO00KKKKKKKKKKK00000000000KKK00OkkkxxkO0KK0kxdddxxkOO00KKK00OOOO0000000KK0kdc::coddddddooooolllllllllllccccoddxkkkxxxdoodddddoxO0000000O0KKKKKKKXXXXXK0kdoooooooooooooooooooooooooooooooloooooooollooooddoodddddddddddoodoooooooolloooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooood
ooooooooooooooooooooooooooooooooodooooooooooooooooooooooooooooooooooooooooooooooooooooooodxkkO0KKKKKKKKKKKKKKKKKKKKKKKKKKKKKXKKK00OOkkxk0KKKOOkkOOO000KKXXKK0OO00KKKKK0KKXK0kol::clooooooolllllllllllllllllllodddoooddxdodddxxdddkO00000000KKKKKXXXXXXKXKOdooooooooooooooooooooooooooooooooooooooollooooodoodddddddddddoodooooodoolloooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooo
oooooooooooooooooooooooodoooooooooooooooooooooooooooooooooooooooooooooooooooooooooooodxkO0KKKKKKKKKKKKKKKKKKKKXKKXXKKKKKKKXXXXXXXK00OkkkOKXK00000KKKKKXXXXKKKKKXXXXXXXXXXXXX0kdlccccllllllllllllllllllllllllllcclllllloddxxxdxdddddkO0000000KKKKKXXKXXXKKK0xoooooooooooooooooooooooooooooooooooooollooddddoodddddddddddddddddddddoolodooooooooooooooooooooooooooooooooooooooooooooooooooooooooooodddoooooooooooo
ooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooddddddoodkO0KKKKKKKKKKKKKKKKKKKKKXXXXXXXXXXXXXXXXNNXNNNXKK00OkO0KXKKKXXXXXXXXNNXXXXNNNNNNNNXXXNNNNNX0kddocccccccccclllllllllllllloddoooollllllooxkxxxdddddxkO00000000KKKKKKKKKKKKOdoooooooooooooooooooooooooooooooooooooolooddddddddddooddddddddoddooddooloooooooooooooooooooooooooooooooooooooooooooooooooooooddddooodddooddooooooood
oooooooooooooooooooooooooooooooooodooooooooooooooooooooooodddooododddddooddooddxkOKXXKXKKKKKKKKKKKKKXXXXXXXXXXXXXXXXNNNNNNNNNNNNNNNNXXXKKKXXXXXXNNXXXXNNNNNNNNNNNNNNNNNNNNNNNNNXK0kdol:;;;:::::ccccccllcccldkkOkxddooooooodxkkkxxxxxxxxkOOOOO00000KKKKKKKKKKOdooooooooooooooooooooooooooooooooooooolloddddddodddooddddddddddddddddooooooooooooooooooooooooooooooooooooooooodddooodddoooddddddddddddddddddooooodd
oooooodddddoooddoooooddooooooooooooooooodddoooddoooooooooooooddddddoooddxxkkO0KXXXXXXXXXXXXXXKXXXXXXXXXXXXXXXXXXXXNNNNNNNNNNNNNNNNNNNNNNNXXXNNNNNNNNNNNNNNNNNNNWWWNNNNNNNNNNNNNXX0Okdolc;;;;;;::::::ccccccldxxxdoooodddddddxxxxxxxxxxxdxxkkOOOOOOO00000KKKKK0kdoooooooooooooooooooooooooooooooooooolloddddddddddddddddddddddddodddooooooooooooodoooooooooooooddooddoooooooooodooodddooodddddddddddddddddddoodddo
dooooddoodddddddoooooooodddddooooooooooooooooodoodddoodoodddddoooddxkO0KXXXXXXXXXXXXXXXXXXKKKK00KK00KKKKKXXXXXXXXXXXNNNNNNNNNNWWWWWWNNNNNNNNNNNNNWWNNNNNNNNNNWWNWWWWWWWNNWNNNNNNXK0kxdolc:;;;;;;;;;;:::ccc:codoooollooooddddddddddddddddddxxkkkkkkOOOOO000KKK0xoooooooooooooooooooooooooooooooodooooloddddddddddddddddddddddddddddoolodoooooddddooodoooooooooddoooooddoodoooddddodddddddddddddoddddddddddddddddo
ddoooooddddddddddddddddddddddddddddddddddoooooddddddooddddddodxkO0KXXXXXXXXXXXXXXXXXXXXXXKKK0Okkkxxk00KKXXXXXXXXNNNNNNNNNWWNNNNWWWWWWWWWWWWWWWNWWWWWNNNNNWWWWNNNWWWWWWWWWWWNNNNNXK0Okxdoolc:;;,,,,,,;;:clldO00OdoooolllllllllllooooooooodoodddxxxxxkkkkOOO0KK0Oxoddooooooooooooooooooooooooooooooooolodddddddddddddddddddddddddddddoooddddddooddooddooooooooooooddodddodddddddddooddddddddddddodddddoddddddddddd
odddddddddddddddddddddddddddddddddddoodddddddddddddddddddodxO0KXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXKK0000KKKXXXXXXXXNNNNNNNNNNNNNNNWWWWWWWWWWWWWWWWWWWWWWNNNNNNNNNNNWWWWWWWWWWWWWWNNXXXK0Okxxddolc:;;,,,,;cdkO0KXXNX0xoooolllllllllllllllllllollllooddddxxxkkOO000000xoodddooooooooooooooooooooooodddddoolodddddddddddddddddddddddddddddoooddooooooddoooodooddddddddddddddddddddddddddddddddddddddddddddddddddddddddd
ddddddddddddddddoddddddddddddddoodddoddddddddddddddddddddk0KXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXKKKKXXXXXXXXXNNNNNNNNNNNNNNNNWWWWWWWWWWWNNNNWWWWWWWNNNNNNNNNNNNNNNWWWWWWNNNNNXXXK00Okkxxddolc:;;;;;:okOO0KXXXXKkdooollllllccccccccccccccclllllooddddxxkOOO000KKkdoddooodddoooodddooooooooooddooooolooddddddddddddddddddddddddddddoooddoooooddddooodddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd
ddddddddddddddddddddddddddddddddoddddddddddddddddddddddk0KKKKXXKXXXXXXKXXXXXXXXXXXXXXXXXXXXXXXXXXXXKKXXXXXXXXXXNNNNNNNNNNNNNNNNNWWWNNNNNNNNNNNNNNNNNNNNNNNXXXXXXNNNNNNNNNNNNNNNNXXXK00OOkkxxxdolc:::::lxkO0KKXXXXX0kdooolllllllccccccccccccccclllloooodddxkkOOO0KKOdoddoooddooooooooooooodoooodoodooooodddddddddddddddddddddddddddddooododddddddddoddddddddodddddddddddddddddddddddddddddddddddddddddddddddddddd
dddddddddddddddddddddddddddddddddddddddddddddddddddddxO0KKKKKKKKKKKKKKXXXXXXXXXKKXXXXXXXXKKXXXXXXXXKKXXXXXXXXNNNNNNNNNNNNNNNNWNNNWWWWWWWWWWWNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNWWNNNNXXKK000OOkkkxxddolcc:codxO0KKKXKXXKKkdoooollllllcccclccccccccccccclllloodxxkOOOOOOkdodddoddddoooodddooooooooodddddoooodddddddddddddddddddddddddddddooodddddddddddoodddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd
ddddddddddddddddddddddddddddddddddddddddddddddddddddxO000KKKKKKKKKKKKKKKKKKKKKKKKXXXKKXXXXXXXXXXXXXXXXXXNNNNNNNNNNNNNNNNNNNNNNNNNNWWWWWWWWWWWWWWWWWWWWNNNNNWWWWWWWWWWWNWWWWWWNNNNXXXKK000OOOOkkxxxddolclodxO00KKKKKKKK0kooooolllllcccccccccccccccc:ccllllodxxxxdoodxxdddooddddooddddddddoooooodddddoolodddddddddddddddddddddddddddddooodddddddddddoddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd
dddddddddddddddddddddddddddddddddddddddddddddddddddxO0000KKKKKKKKKKKKKKKKKKKKKKKKKKKKKXKKXXXXXXXXXXXXNNNNNNNNNNNNNNNNNNNNNNNNNNNNWWWWWWWWWWWWWWWWWWWWWWWWWNWWWWWWWWWWWWWWWWNNNNNNNXXXK0000OOOOkkkkxxddolloxkO0000KKKKKK0Oxdoloollllcccccccccccccc::::cccllllolllccclxxdododddooddddddddooddoooodddddolodddddddddddddddddddddddddddddoooddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd
ddddddddddddddddddddddddddddddddddddddddddddddddddxkOO00000KKKKKKKKKKKKKKKKKKKKKKKKKKKXXXXXXXXXXXXXXXNNNNNNNNNNNNNNNNNNNNNNNNNNNNNWWWWWWWWWWWWWWWWWWWWWWWWNNNNNNNNNWWWWWWWWNNNNNNNXXXKK0000OOOOkkkkxxddooddxkO00000KKK00Okxxddolllllllllllcccccccccccclllcccc::cccccdkxddddddooodddddddddddoooddddddooodddddddddddddddddddddddddddddolodddddddddddoodddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd
dddddddddddddddddddddddddddddddddddddddddddddddddxkO000000KKKKKKKKKKKKKKK0KKKKKKKKKKKXXXXXXXXNNXXXXXXNNNNNNNNNNNNNNNNNNNNNNNNNNNNNWWWWWNNNNNNNNNWWWWWWWWNNNNNNNNNNNNNNNNNNNNNNNNNNXXXXKK000OOOOOkkkkkxddoddxkOO00000KKKOkxxkOOkkxdoollllllccccccccccccccc::;;::ccllldxkxddddddodddddddddddddooddddddoooddddddddddddddddddddddddddddddooddddddddddddodddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd
dddddddddddddddddddddddddddddddddddddddddddddddddxkOO00000KKKKKKKKKKK0000000KKKKKKKXXXXXXXXXXXXXXXXXXNXXXXNNNNNNNNNNNNNNNNNNNNNNNNNNWWNNNNNNNNNNNNNNNNNNNXXXXXXXXXXXXXXNNNNNNNNNNXXXXKKK0000OOOOOkkkkxdddddxkkOO00000KK0kxxkOO000KK0Oxdlllllcccc::::::::;;;;;;;:clooodxxddddddddddddddddddddooddddddoooodddddddddddddddddddddddddddddooddddddddddddodddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd
ddddddddddddddddddddddddddddddddddddddddddddddddxkOOO000000000000000000000000000KKXXXXXXXXXXXXXXXXXXXXXXXXXXNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNXXXXKKKKKKKKKKKKKKKKXXXXXNNNNNNNNNXXXXXXKKK000OOOOOOkkkxxddddxxkkOO000KKKKOxxkO00KKKKKXXK0kdlllcccc::::;;;;;;;;;;:cloodddxxdddddddddddddddddddooddddddoooodddddddddddddddddddddddddddddooddddddddddddodddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd
dddddddddddddddddddddddddddddddddddddddddddddddxkkOOO00000000000000000000000000KKKXXXXXXXXXXXXXXXXXXXXXXXXXXXXXNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNXXXKK00O000000KKKXXXXXNNNNNNNNNNXXXXXXXKKK00000OOOkkkkxxxddddxxkOO0000KKK0kkO0KKKKKKXXXXXXKxllccc:::::::;;;;;;;;;:clodddxkkddddddddddddddddddooddddddooooddddddddddddddddddddddddddddddoodddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd
dddddddddddddddddddddddddddddddddddddddddddddddxkOOOOO000000000000000000000000KKKKXXXXXXXXXXXXXXXXXXXXXXXXXXXNXXXXXXNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNXKKK000KKXXXXNNNNNNNNNNNNNNXXXXXXKKKKK0000OOOOkkkxxxxxdddxkOOO00K000K0O0KKKKKKKXXXXXXXKxllccc::::::::;;;;;;;::coddxxkOkdddddddddddddddddoodddddddoooddddddddddddddddddddddddddddddoodddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd
ddddddddddddddddddddddddddddddddddddddddddddddxkkOOOOOOO0000000000000000000KKKKKKKKXXXXXXKKKKKKKXXXXXXXXXXXXNNXXXXXNNNNNNNNNNNNNNNNNNNNNNNNNNNNWWWWWWNNXXXXXXXXXXXNNNNNNNNNNNNXXXXXXXXKKKK0000OOOOkkkkxxxxdddxxkOO000000KK00KKKKKKKKKXXXXXNKxlccc:::::::::;;;;;;;:clodxxkkkkdddddddddddddddddoddddddddooddddddddddddddddddddddddddddddoodddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd
ddddddddddddddddddddddddddddddddddddddddddddddxkkkOOOOO0000000000000000000000000KKKKKKKKKKKKKKKKKKKKXXXXXXXXXXXXXXNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNWWWWNNNNNNXXXXXXXXXXNNNNNXXNNNXXXXXXXXKKKK0000OOOOkkkkkxxxxxxxxkOOO0000000000KKKKKKKKKXXXXXXKxllc:::::::::;;;:;;;;:codxkkkkkkddddddddddddddddoodddddddooddddddddddddddddddddddddddddddoodddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddx
dddddddddddddddddddddddddddddddddddddddddddddxxkkkkOOOOO000000000000000OO000000000000000KKKKKKKKKKKKXXXXXXXXXXXXXXNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNXXKKKKXXXXXXXXLOVEXYOUXFOREVERK0000OOOOkkkkxxxxxxxxxkkOOOO000000000000KKKKKKXXXXXX0dllc:::::::::;;;;;;;;:lodxkkkkkkxddddddddddddddoodddddddooddddddddddddddddddddddddddddddoodddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd
dddddddddddddddddddddddddddddddddddddddddddddxxkkkkOOOOO000000000O0OOOOOOOOOOOOOOOOOO00000000000KKKKKKKKXXXXXXXXXXXXXXXXXXXXXXXXXXXXNNNNNNNNNNNNNNNNNNXXNNNXKK00KKKXXXXXXXXXXXXXXXXKKKKKKKK0000OOOkkkkkxxxxxxxxkkOOOOO00000O0000000KKKKKKXXXXKxllcc::::;;;;::::;;;;::codxxxxkOkddddddddddddddoodddddddooddddddddddddddddddddddddddddddooddddddddddddddddddddddddddddddddddddddddddddddddddddddxxdddddddddddddddd
dddddddddddddddddddddddddddddddddddddddddddddxxkkkOOOOO000000000OOOOOOOOkkkxxkkkkkkkkOOOOOOO000000KKKKKKKKXXXXXXXXXXXXXXXXXXXXXXXXXXXNNNNNNNNNNNNNNNNNXNNNNXKK0KKKXXXXXXXXXXXXXXXXKKKKKKKK00000OOOOkkkkxxxxxxxxkkkOOOOOOOOkkOOOOO0000KKKKKKKKKxllccc::::;;:::;;;;;;;::cloodxxkkxddddddddddddddddddddddooodddddddddddddddddddddddddddddoodddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd
dddddddddddddddddddddddddddddddddddddddddddddxxxkkOOOOOO0000000OOOOOOOOOkkxdddxxxxddxxkkkOOOOO000000KKKKKKKXXXXXKXXXXXXXXXXXXXXXXXXXXXNNNNNNNNNNNNNNNNNNNNNNXXXXXXXXXXXXXXXXXXXXXXKKKKKKKKK0000OOOOkkkxxxxxxxxxxxkOOkkOkkkddxkkkkOO000000KKKKKklllccc::::::::;;;;;;;;;;:cloddxkkxddddddddddddddddddddddoodddddddddddddddddddddddddxddxdodddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd
ddddddddddddddddddddddddddddddddddddddddddddddxxkkkkOOOOOO000OOOOOOOOOOkkxollllooolodxxxxkkOOOOO000000KKKKKKKKXKKKKKXXXXXXXXXXXXXXXXXXNNNNNNNNNNNNNNNNNNNNNNNNXXXXXXXXXXXXXXXXXXXKKKKKKKKK0000OOOOOkkkxxxxxdlccoxkkkkkkxxdlldxxxxkkOOO0000000Kkollccc:::::::::::;;;;;;;;;:codxkkkxddddddddddddoddddddddoodddddddddddddddddddddddddxxxxdooddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd
dddddddddddddddddddddddddddddddddddddddddddddddxxkkkkOOOOOOOOO000000OOOOkkdlcccllccloddxxxkkkkOOO0000000KKKKKKKKKKKKKXXXXXXXXXXXXXXXXXXXXNNNNNNNNNNNNNNNNNNNNXXKKKXXXXXXXXXKKKKKKKKKKKKKKK0000OOOOkkkkxxxxdol:codxkkkkxdo:;clodddxxkkOO0000KKK0dlllcc:::;;:::::::;;;;;;;;;:cldxkOkxdddddddddddoddddddddoodddddddddddddddddddddddddxxxxdooddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd
dddddddddddxxddddddddddddddddddddddddddddddddddxxxxkkkkOOOOOO000000OOOOOOOkdcccccccloodddxxxkkkkOOOOOO00000KKKKKKKKKKKKKKKKKKKKXXXXXXXXXXXXNNNNNNNNNNNNNNNNNNXXKKKKKKKKXKKKKKKKKKKKKKKKK0000000OOOkkkxxxxxddollodxxxxxdl:',;cclooddxxkkOO000KKKkollccc:::::;;;;::;;;;;;;,;;;cldxkOOkdddddddddddddddddddoodddddddddddddddddddddddddddxxdooddddddddddddddddddddddddddddddxxxxddddddddddddddddddddddddddddddddxxxxx
ddddddddddddddxxdddddddddddxdddddddxdddddxdooddddxxxkkkkkOOOO000000OOO000OOkocccccclooddddxxxxkkkkkkOOOOO00KKKKKXKKKKKKKKKKKKKXXXXXXXXXXXXXXXXXXNNNNNNNNNNNNXXXKKKKKKKKKKKKKKKKKKKKK000000000OOOOkkkkxxxddxdoooddxxxxdl;'',;;:cclooodxxkkOO000K0dllcccc::::::::;;;;;;;;;;;;;;:ldkO00kddddddddddddddddddoodddxxdxdddddddddddddxxxdddddxdoodxdddddddddxddddddddddddddddddddddddddddddddddddddddddddxxddxxxxxxxxxxx
dddddddddddddxxdddddddddddddddddddxxdddddddoooodddxxxkkkkkOOOO0000000000000Oxoccc:clllooodddxxxkkkkkkkkkkkkO0KK000000KKKKKKKKKKKKXXXXXXXXXXXXXXXXXXXXXNNXXXNNXKK000KKKKKKKKKKKKKKKK00000000OOOOOkkkkxxddddddoooddddddl;'.',,;::ccllooodxxkkkOO00koccccc::::::::;;;:;;;;;;;;;;;:ldkO00Oxddddddddddddddddooddddxxddddddxxddxddxxxxxxxxdxddodxxdddddddddddddddddddxddddddddddddddddddddddddddxxxxxxxxxxxxxxxxxxxxxx
xdddxdddxxddddddddddddddddddddxxxxxxdddddxdooooddddxxxxkkkOOOO00000000KKKK00Oxl::ccllllooooodddxxxxxxxkxxxdooooxkkOOO000000KKKKKKKKKKKKXXXXXXXXXXXXXXXXXXXXNNXXK0000000KKKKKKKKKKK00000OOOOOOOOkkkxxxdddddooooooodddl'...',;;;::cccllloodddxxkkOOdlcccc::::::;;;;:::;;;;;;;;;;;:ldkOO0Oxddddddddddddddddodxxdxxdxxxxxxxxxxxxxxxdxxxxxxddodxxddddddddddddddddddddddddddddddddddxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
xxxxxdddxxxxxdddddddddddddddxxxddddddddddddooooodddddxxxkkOOOOO00000KKKKKKK00koc::ccllllllllooodddddxxxxxdoc::ldkOOO000000000KKKKKKKKKKKKKKKKKXXXXXXXXXXXXXXNXXXK00000000000000000000OOOOOOkkkkkkxxdddddoooooooodool,....',;;;;::cccccclllooddxxkxocccc::::::;;;;::;;;;;;;;;;;;;:lxkOOOOkdddddddddddddddoodxdxxxxxdxxxxxxxxxxxxxxxxxxxxdodxxdddddddddddddddddddddddddddddxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
ddddddxxxxxxxxxxxxdxxxxxxxxxxxxxxxxxxxxddddooooooodddxxxkkkkOOOO000KKKKKKKKK00xl:;:ccllccccclllooodddddxxxdlcldkOOO0O00OOOO000000000KKKKKKKKKKKKKKKKKKKKKKXXNNNXKK000OO00000000000000OOOOkkkkkkkxxxdddooooooooooooo:',;''',;;;;:::ccccccccllloddxxocccc::::::::;;:;::;;;;;;;;;;;;:lxkkkOOkxdddddddddddddoodxxxxxxdddxxxxxxxxxxxxxxxxxxxdooxxddddddddddddddddxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
dddddxxxddxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxdolloooooddxxxkkkkkOOO000KKKKKKKKKK0kd:,;:ccccccccccclloooodddddddodxkkkkOOOOOOOOOOOOOO000000000KKKKKKKKKKKKKKKKXXNNNNXXK00OOOOOOOO0OO000OOOOOOkkkkkxxxxdddooooollllloool;':oc;,,;;::::::cccccccccllooddocccc:::::::::;;;;;;;;;;;;;;;;;;coxxkkkOkxddddddddddddoodxdxxxxxxxxxxxxxxxxxxxxxxxxxxxddxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
xxdxxxxxxxxxxxddxxxxxxxxxxxxxxdxxxdxxxxxxxdollooooooddxxxkkkkOOO00KKKKKKKKKKK0Oxc,',:ccccccccccccllllooooooodddxxxxkkkkkkkkkOOOOOO00000000000KKKKKKKKKKKKXNNNNNNNXXXK00OOOOOO0OOOO0000OOOOkkkkkxxxddddooooooolooodl,':ddl:;;;::::::ccccccccllllooolcccccc::::::;;;;;;;;;;;;;;;;;;;:loodxxkkkddxdddddddddoodddxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
xxxxxxxxxxxxxxdxxxxxxxxxxxxxxxdxxxxxxxxxxxdolllooooooddxxxkkOO0000KKKKKKKKKKK0Oxl;..';::ccccccccccccllllloooooddddxxxxxxxkkkkkOOOOO0000000000KKKKKK0KKKXXNNNNNNNNNNNNNXXKKKKKKKKKKKKKK0000OOOOOkkxxddddddooooooodddc,cddddl:;:::::cccccccccclllloolcccccc::::::::;;;;;;;::;;;;;;;;;:ccloddxkxdddddddddddddxxxxxxxxxxxxxxxxxxxxxxxdxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxdxxxxxxxxxdollloooooooddxkOOO000KKKKKKKKKKKK00Oko:,..',;::::ccccccccccccllllooooddddxxxxxxxkkkkOOOOO0000KKKKKKKKXXXXXNNNNNWNNNNNNNNNNNNNXXXXXXXXXXXXXKKKKK0000OOOkkxxxdddddodddddxko:cdxdddoc::::cccccccccccllloddllccccc::::::::;;;;;;;::;;;;;;;;;;::cllodxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxolllooodddddxkO0000KKKKKKKKKKKKKK00Oko:,...'',;;::ccccccccccccccllllooooddddxxxxxkkkOOOO000KKKKXXXXXNNNNNNNNNNNNNNNNNNNNNNNNNNNXXXXXXXXXXXXKKKKKK00000OOkkkxxxxddddxxxkOklcdxdddddlc::cccccccccccclodxdlcccccc:::::::;;:;;;;;;;;;;;;;;;;;;;;:cloodxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxdooooddxxxkkOO000KKKKKKKKKKKK00K000Okdc,...'',,;;:cccccccccccccccccllllloodddxxxkkkOO000KKKKXXXXXXXXNNNNNNNNNNNNNNNNWWNNNNNNNNXXXXXXXXXXXXXKKKKKKK00000OOOkkkkxxxxxxxkkOOdldxdddddddlccccccccccccllodxdlccccc::::;;;;;::;;;;;;;;;;;;;;;;;;;;;:ccloxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxdddddxkkOO00000K0KKKKKKKKKK0K000000Oxdc,...',,,,;;::cccccccccccccccllloooddxxkkOOOO0000KKKKKKKKXXXXXNNNNNNNNNNNNNNNNNNNNNNNNNNNXNNXXXXXXXXKKKKKKK000000OOOOkkkkkkkkkxkkkkxodxxxxxxxxxdlccllccccccllodddlcccccc::::::::::;;;;;;;;;;;;;;;::;;;;;;:codxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxkkkOOO00K00KKKKKKKKKKKK00K00000OOxdc,..''',,,;;::cccccccccclllllooodxxkkOOOO000000000KKKKKKKXXXXNNNNNNNNNWWWWNNNNNNNNXXXNXXXXNNNNNXXXXKKKKKKK000000OOOOOOkkkkkxkkxxxkkddxxxxxxxxxxxxdollllcccccllooolccccccc:::::::::;;;;;;;;;;;;;;;;::;;,,;;:ldxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxkkkOOOO000000KKKKKKKK000000000000Okxoc'..''',,,;;::cccccclllllooodddxkkOOO0000000000KKKKKKXXXXXXNNNNNNNWNNNWWNNNNXXXXXXXKXXXXXXXXXXXXXXXKKKKKKK0000000OOOOOOkkkxxxxxxxxxxdxxxxxxxxxxxxxxdollllcccclllolcccccccc::::::::::::;;;;;;;;;;;;;::;;;;;:cdxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxkkOOOO00000000KKKKKKK0000000000000Okxo:'.''',,,;;:::ccclllooooodddxxkkOOOO000000000KKKKXXXXXXXXXXXNNNNNNNNNNNNNXXXXKKKKKKKK00KKKKKKKKKKKKKKKKKKKKK0000000OOOkkkkxxxxxxxxxxxxxxxxxxxxxxxxxxxdolllcccllllllccccccc:::::::::;;;;;;;;;;;;;;;;;::;;;;:cdxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxkkOO00000000000KKKKKKKKK0000000000OOkxo:'.''',,,;;::cclllooooddddxxxkkkkOOOO0000KKKKKKXXXXXXXXXXXXXXXXXXXXXXXXXXXXKK000OOOO000000K0000KKKKKKKKKKKKKK000000OOOkkkkxxxxxxxxxkkxxxxxxxxxxxxxxxxxxdollllccllllcccccccccc:::::::;;;;;;;;;;;;;;;;:::ccloodxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxkOO0000000000000000K00KK000000000OOkkxl;,'''',,;;::cclloooooddddxxxxxkkkkOOOO000KKKKKKKKXXXXXKKKKKKKKKKKKKKKKKKKKKK0000OOOOO00000KK0000000K000KKKK000000OOOOkkkkkxxxxxxxxxkkxxxxxxxxxxxxxxxxxxxxddolccclccccccccccc:::::::::;;;;;;;;;;;;;;;::clodxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxkkOO000000000000000000000000000000OOOkdl;;,,,,,,;;::ccloooooodddddxxxxxxxkkOOOOO0000000KKKKKKKKKKKKKKKKKK0KKKKKKKK0000KKKK0000KKKKKKKK000000000000000000OOOOOkkkkkkxxxxxxxxkkxxxxxxxxxxxxxxxxxxxxxxxollcccccccccccc:::::::::::::;;;;;;;;;;;;::clloxkxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxkkOO000000000000000000000000000000OOkxdl::;;,,,;;;::cloooooooddddddddxxxxkkkkkOOOO0000000000000000000000000KKKKKKKKKKKKXXKKKKKKKKKKKKKKKK0000000000OOOOOOOOkkkkkkkkkxxxxxxkkkxxxxxxxxxxxxxxxxxxxxxxxxdolccccccccc::::::::::::::::;;;;;;;;;;;;::ccloxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxddddoool
xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxkOOO000OOOOO00000000000000000000OOOkxdlc::;;;;;;;::clooooooooddddddddddxxkkkkkOOOO0000OOOOOOOOOOOOO000000KKKKKKKXXXXXXXXXXKKKKKKKKKKKKKKKK0000000OOOOOkkkkkkkkkkkkkkxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxdocccccccc::cccc:::::::::;;;;;;;;;;;;;;;;:cloxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxddddooollllllcccc
xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxddxkkOOOOOOOOOOO0000000000000000000OOOkxdolcc::;;;;;:cllloooddddddxxxxxxxxxxxxkkkkOO0000OOOOOOkkkkkkOOO0000KKKKKKKXXXXXXXXXXXKKKKKKKKKKKKKKKKK0000000OOOOOkkOkkkkkkkkkkkkxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxolccccccccc::::::::::::;;;;;;;;;;,;;;;;;:clodxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxddddoooolllllcccccccccccccl
xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxdddxkkkOOOOOOOOOO00000000000K000000OOOkxdxdlcc::;;;;:cclloooddddxxxxkkxxxxxxxxkkkOOOOOO000OOOOOkkkOOOO0000KKKKKKKKKKXXXXXXXKKKKKKKKKKK0000000000000000OOOOOOOOOkkkkkkkkkkkkxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxdlccccccc::::::::::::;;;;;;;;;;;;;;;;;;;:coxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxkxxxxxxxxxddddoooolllllllccccccccccclllllllloooo
xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxooodxxkkkOkkkkOOOOO00000000KK000000OOkxdxxxdocc:::;::ccclllooddxxxkkkkkkkkkkkkkkkkOOOOOOOOOOOOOOOOOO000000000KKKKKKKKKKKKKKKKKKK000000000000000000000OOOOOOkkkkkkkkkkkkkkkkxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxdllccccccc:::::;;;::::;;;;;;;;;;;;;;;;;cdxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxkkxxxxxxxdddddoooolllllccccccccccccllllllllllooooodddooooooo
xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxooodxxkkkkkkkkkOOOOO00000000000000OOOkxdxxxxxolc::::::ccclloodddxxkkkkkkOOOOOkkkkkkkkkkkkkkkkkkkOOOO00000000000000000KKKKKKKKKK00000000000000000000OOOOOOkkkkkkkkkkxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxdollccc::::::::;;:;:;;;;;;;;;;;;;;;;cdxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxdddddooooolllllllcccccccccccllllllllooooddddddooooooooooooodddd
kkkkkxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxooodxxkkkkkkkkkOOOOO00000000000000OOOkxdxxxxxxdlc:c:::cccclloodddxxxxkkkOOOOOkkxxxxxxxxxkxxxxxxxkkOOOOOOOOOOO000000000000000000OOkOOOOOOO00000000OOOOOkkkkkxxxxxxxxxxxdddddddxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxolccc::::::::::::::;;;;;;;;;;::;cdxxxxxxxxxxxxxxxxxxxxxxxkxxxxkxxxxxxxxxxddddooooolllllllcccccccccccllllllllloooooodddddooooooooooooooodddxxxkkkkkkkk
xxxxxxxkkkkkkkkkkkkkkkkkkkkkxxxxxxxxxxxdoddxxkkkkkkkkkOOOOO0000000000000OOOkkxdxxxxxxxxolcc::ccllllloooodddxxxkkkOOOkkxxxddddddddddddxxxkkOOOOOOOOOOOOOOOOOOOOOO000OOkkkxxkkkkOOOOOOOOOOOOOOkkkxxxddoooodddddddoooodddxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxdolccc::::::::::::;;;;;;;;;;::cdxxxxxxxxxkxxxxxxxxxxxxxddddddooooollllllllccccccccccclccllllllllloooodddddddooooooooooooodddddxxxxkkkkkkkkkkkkkkkkkk
llllooooooddddddddxxxxxxxxkkkkkkkkxkkkxdddxxxkkkkkkkkkOOOOOOO00000000000OOkkxdxxxxxxxxxxdocc:clllllllllooooddxxxxkkkkkxxddoooooooooddxxxkkkkkkkkkkOOOOOkkOOOOOOOOOOOkkkxxxxxxxkkkkkkOOOOOkkkkxxxdddoollooooooooollooodxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxdolc::::::::::::::;;;;;;;;:coxxxxxxxxddddoooooooollllllllcccccccccccclclllllllllllooooddddddooooooooooooooooddddxxxkkkkkkkkkkkkkkkkkkkkkkkkkkxxxdd
llllllcccclllllllllllooooooooddddddddddddxxxkkkkkkkkkkOOOOOOOO00000000OOOkkkxdxxxxxxxxxxxxoccccllllllllllllooodddxxxxxddooolllllllooddxxxkkkkkkkkkkkkkkkkkkkkkkkOOOOOkkxxxxxxxkkkkkkkkkkkkxxxxdddoooolllllloooooooooodxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxolc:::::::::::::;;::;;;:cloooolllllllccccccccclcclcllccllllllllloooooodddddddooooooooooooooddddddxxxkkkkkkkkkkkkkkkkkkkkkkkkkkkxxxxxddooollcccc
dddddooooooooollllllllllcccccllllllllooddxxxxkkkkkkkkkOOOOOOOOOO000000OOOOkxdxxxxkkkkkkkkkxoccclcccccllllllllllooooooooolllllcccclloddxxxxkkkkkkkkkkkkkkkkkkkkkkkOOOOOkkkkkkkkkkkkkOOkkkkkxxxdddddoooolllllloooooodddxkkxxxkkxxkkxkkkkxxxxxxxxxxxxxxxxxddddddddddoolcccc::::::::::;;;;;;:cccccccccllllllllllloooooooooodddddddddoooooooooooooodddddddxxxxkkkkkkkkkkkkkkkkkkkkkkkkkkkkxxxxddddooolllccc::::::::::
ddddddddxxxxxxxdddddooooooooooooooollodddxxxxkkkkkkkkkOOOOOOOOOOOO000OOkkkxxdoodddddddddddddlcccclcclllllllllllllllllollllllccccclooddxxxxkkkkkkkkkkxkkkkkkkkOOOOOOOOOOOkkkOOOOOOOOOOOOkkkkxxxxdddooodddoooooodddddddxxxxxxxxdddddddddoooooooooooolllllllllllllllclccllcccccccccc:;;::cllooooooooooodddddddddddddooooooooodddodddddddddxxxxkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkxxxddddoolllccc::::::::::::::::::::::
dddddddddddddddddoooodddddddddxxddxxddddxxxxkkkkkkkkkkkOkOOOOOOOOO000OOkkkxxolcllllllllllllllcccclllllllllllooooooooooooooolllcclloodddxxxkkkkkkkkkkkkkOOOOOOOOOOOOOOOOOOOOO000000OOOOOOOOOkkkxxddooddxxxxxdddddddddollllllllllllllllllllllccccllcccllllllllllllloolooooooooooodoooooodxxxddddooooooooooodddddddddddddddxxxxkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkxxxxxdddoolllcccc:::::::::::::::::::::::::::::::::
kkkkkkkkkkkkkxxxxxdddddddddddddddddddoddxxxxkkkkkkkkkkkkkkOOOOOOOOOOOOkkxxxddoooooooooooooollllccclllllllllooooooooddddxddddoolloooddddxxxxxxxkkkkkkkOOOO00O000000000000000KKK000000000OOOOOkkkkxxddddddxxxxddddddddollllllllllooooooooooooooooooooooooodddddddddddddddddddooooooooooooddddddddddddddddxxxxkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkxxxxxdddooolllccccc::::::::::::::::::::::::::::::::::::::::::::
kkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkxxxxxdddxxxxkkkkOOOkkxxxkkkkkOOOOOOOkkkkxxddddxdddxxxxxxxxxdddolccllllllllooooooodddddddxxdddddddddddddxxxxxxxxkkkOOOO0000000000KK00KKKKKKKKKKKKKKK000000O0OOOOkkkxxddooddddddddddooddddddxxddxxxxxxxxxxddddddddoooddoooooooooodddddddddddddddddddxxxxxxxkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkxxxxxdddoooolllcccc::::::::::::::::::::::::::::::::::::::::::::::::::::::::c:
xxkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkxdddxxxkkkOOOOOkkkxxxxxkkkkkkkkkkkkkxxddddddddddddddddddddoolccllllllloooooodddddddddddddddddddddddddxxxxxkkOO0000KKKKKKKKKKKKKKKXXKKKKKKKKKKKKK0000000OOOOkkkkxxdololloooooooloddoooooooododdddddddddddddddddddddddxdxxxxxxxkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkxxxxdddoooollllcccccc::::::::::::::::::::::::::::::::::::::::::::::::::::::::::::::ccccccc::
cllllloooodddddxxxxxkkkkkkkkkkkkkkkkxdddxxxkkOOOOOOOkkkxxxxxkkkkkkkkxxxxdddxkkkkkkkkkkxxxxxxxxxxdllllloooooooddddddddddddddoooooododdddddxxkkOO00KKKKKKXXXXXXXXXXXXXXXXKKKKKKKKKKKKK000000OOOOOkkkkxxdolllclclllllodddddddddxxxxxxxxkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkxxxxxxxdddddooollllccccccc::::::::::::::::::::::::::::::::::::::::::::::::::::::::c:::cccccccccccccccccc::cc::::
::::::::::::ccccccccclllloooooddddxxdoddxxkOOOOO000OOOkxxxxxxkkkkkkxxxxddddkkkkkkkkkkkkkkkkkkkkkkdolloodddxxxxxxddddddddooooooooddoodddxkkOO00KKKKKKXXXXXXXXXXXXXXXXKKKKKKKXKKKKKKKKK00000OOOOOkkkxxxddoollccccclloxkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkxxxxxxddddoooolllllcccccccc::::::::::::::::::::::::::::::::::::::::::::::::::::::::::::::::::cccccccccccccccccc::::::::::ccccccccc
::::::::::::::::::::::::::::::::::cclodxxkkOO00000000OOkxxxxxxxkkkkxxxddodxkkkkkkkkkkkkkkkkkkkkkkkxoloddxxxkkkOOkkkkkkkxxxxxxxxxxxxxxkkOO00KKKKKKKKKKKKKXXXXXXXXXXKKKKKKKKKKKKKKKKKKK00000OOOOkkkkxxddoooollccllllokkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkxxxxxxxxxxddddddoooooolllllccccccc:::::::::::::::::::::::::::::::::::::::::::::::::::::::::::::::::::::ccc::ccccccccccccccccccccc::::::::cc:::cccccccccccccccc
:::::::::::::::::::::::::::::::::::clddxkkOO000KKKKKK00OkxxxxxxkkkxxxddolllloolooooooodddddddddxxxxdoodxxkkkO000000000000000000OOkkkkOO000KKKKKKKKKKKKKKKKXXXXXXKKKKKKKKKKKKKKKKKKKKKK0000OOOOkkkxxxddoooolllllllloxxxxxxdddxdddddddddoooolllllllllllccccccccccc:::::::::::::::::c:c::::::::::::::::::::::::::::::::::::::::::::::::::::cccccccccccccccccccccccccccccccc:::::::::::cccccccccccccccccccccccccc:::
:::::::::::::::::::::::::::::::::::clddxkO000KKKKKKKKKK0Okxxxxxxkkkxxddocc:::::::::::::::::cccccccccldxxkOOO00KKKKKKKKKXXXXXXXKK00OOOO000KKKKKKKXKKKXXXXKKKXXXXKKKKKKKKKKKKKKKKKKKKKKKKK000OOkkkxxxxxdddooollllclccccccccccc:::::cc:::::::::::::c::::::::::::::::cc::::::::::::::::::::::::::::::::::::::::::::::cc:::::::ccccc:ccccccccccccccccccccccccc::::::::ccc::ccccccccccccccccccccccccccc:::::::::::::::
ccccccccccc::::::c::::::::::::::::ccodxkO0000KKKKKKXXXKK0Okxxxxxxkkxddol:::::::::::::::::::::::::cc:ldxkOO000KKKKKKXXXXXXXXXXXXXKK00000KKKKXXXXXXXXXXXXXXXXXXXXKKKK000KKXXXXXXXXXXXXXKKKKK00Okkkxxxxxxxxxddoollllc::::::ccc::::::::::::::::::::::::::::::::::::::::::::::::::::::::::::::::::cc::c::::cc:ccccccccccccccccccccccccccccccccccc:::ccccccccccccccccccccccccccccccccccccc::cc::::::::::::cccccccccccc
ccccccccccccccccccccccccccccccccccccodkO00KKKKKXXXXXXXXKK0Okxxxxxkxxdolc:::::::::::::::::::::::::c::lxkOO000KKKKKKXXXXXXXXNNNNNXXXKKKKKKXXXXXXXXXXXXXXXXXXXXXKKKK00000KXXXXXNNNNXXXXXXXXKKK00OOkxxxxkkkkkkxxddoolc:::::::::::::::::::::::::::::::::::::::::::::::::::::::::::::::::::ccccccccccccccccccccccccccccccccccccccccccc::ccccccccccccccccccccccccccccccccccccc:cc:ccc:c::::::::cccccccccllllllllllllooo
cccccc:ccccccc::cccccccccccccccccccloxO0KKKKKXXXXXXXXXXXKKOkkkkkkkxxdoccc:::::::c::c:::::::::::ccc:lxOOO00KKKKKKKXXXXXXXXXXNNNNXXXKKKXXXXXXXXXXXXXXXXNNNNNNNXXXXK0OO0KKXXXXNNNNNNXXXXXXXXKKK00OOkkkkOOOOkkkkxxddocc:::::::::::::c::::::::::::::::c::::::::cc::cccccccccccccccccccccccccccccccccccccccc:ccccccccccccccccccccccccccccccccccccccccccccccccccccc:cc:::c::::::::ccccccccclllclllllllloooooooooooooodd
cccccccccccccccccccccccccccccccccccldkO00KKKKXXXXXXXXXXXXK0kkkkkkkxxolccccccccccccccccccccccccccccokO0000KKKKKKKXXXXXXXXXXXXNNXXXXKKKKKKXXXXXXXXXXXXXXNNNNNNNNNXXXXXXXXXXXXXNNNNNNNXXXXXXXKKK00OOkkOOOOOOOOkkkxxoccccccccc:ccccccccccccccccccccccccccccccccccccccccccccccccccccccccc::cccccccccccccccccccccccccccccccccccccclcccccccccccccc::cc::c:::cccc::ccccccccccclllllllllllllooooooooooooooddddddddddddddd
ccccccccccclccllcccllcccccccccccccloxkO000KKKKXXXXXXXXXXXKK0OOOOOkkxocccccccccccccccccccccccccccldkO0000KKKKKKKKXXXXXXXXXXXXXXXXXKKKKKKKKKKKKXXXXXXXXXXXXXNNNXXXXK0OOOO0KXXXXXXXXXXXXXXXXKKKK000OOOO000000OOOkkkdlcccccccccccccccccccccccccccccccccccccccc:cccccccccccccccccccccccccccccccccccccccccllcccccccccccccccccccccccccccccccccccccccc::cccccccclccllllllllloooooooooooooodddddddddddddddddddddxxxxxxxxx
cccccccccccccccccccccccccccccclcclloxkOO00KKKKXXXXNNXXXXXXXKK0000OOxlcccccccccccccccccccccccccccok0000KKKKKKKKKKXXXXXXXXXXXXXXXXXKKK0KKKKKKKKKKKKKKKXXXXXXXXXXXXXNXOddxkOKXXXXXXXXXXKXXXKKKKK000OOO0000000OOOOOOxocccccccccccccccccccccc:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccclclllllllllllloooooooooooodddddddddddddddddddxxxxxxxxxxxxxxxxxxxxxxxxxxxx
llllllllllcccccccccccccccccccccccclooxkO000KKKXXXXXXXXXXXXXXXXXXK0Odlllllcccccccccccccccccccccldk0000KKKKKKKKKXXXXXXXXXXXXXXXXXXXKKKK00KKKK0000KKKKKKKKKKXXXXXNNNNNXKKXNNNNXXXXKKKKKKKKKKKKKK000OOO0000000OOOOOOkdlcccccccclcccccccccccccccccccccccccccclccccccclllcccccccccccccccccccccccccccccccccccccccccccccccllllcclllllllllloooooooooooooooooddddddddddddddddddddxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxk
ddddddddoooooollolllllllllllllllllllloxkO00KKKKKXXXXXXXXXXXXNNXXK0OdlllllccllccllllllllllllllldO00KKKKKKKXXXXXXXXXXXXXXXXNNNNNXXXXKKKKKK000000000KKKKKKKKKKXXXXXNNNNNNNNNNXXXKKKKKKKKKKKKKKK000OOOO0000000OOOOOOOkolllllllcllllcclllccclcccccccccccccccccccccccccccccccccccccccccccccccccccccccclllllllllllllloooooooooodddooodddddddddddddddddddddxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxkxxxxxkkkkkkkkkkkkkkkkk
kkxxxxxxxxxxxxxxxxxxdddddddddddoooollldxkO000KKKXXXXXXXXXXXXXXKKKK0dllllllllllllllllllllllllldOO00KKKKKKXXXXXXXXXXXXXNXXNNNNNNNXXXXKKKKK000000000000KKKKKKKKKKXXXXXNNNXXXXXKKKKKKK0KKKKKK00000OOOOO00000000OOOOOkkdlccccccccccccccccccccccccccccccccccccccccccccclllllllllllllllllllllllloooooooooooooooddddddddddddddddddxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxkkxxxkkkkkxkkkkxxkkkkkkkkkkkkkkkkkk
kkkkkkkkkkkkkkkkkkxkxkxxxxxxxxxxxxdollodxO0000KKKXXXXXXXXXXXXXXKKK0xoooooooollollllllllllllldkO000000KKKKXXXXXXXXNNNNNNNNNNNNNXXXXXXKKKKK00000000000000000KKKKKKXXXXXXXXXKKKKKKK0000000000000OOOOOO00K00000OOOOOkkxolllllllllllllllllllllllllllllllllllloooooooooooooooooooooddddddddddddddddddddddddxdddxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxkkxxxxxkkxxkkkkkkxkkkkkkkkkxxxxkkkkxkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkk
kkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkxdooodxkOO000KKKXXXXXXXXXXXXXXKKKOxxxxxxxxddddddddddddddodxkO0000000KKKKXXXXXXXXXNNNNNNNNNNNXXXXXXXKKKKK000000000000000000KKKKKKKKKKKKKKKKK0000000000000OOOOOOOOO00KK0000OOOOOOkkdoooooooooooooooooooodddddddddddddddddddddddddddddddddxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxkkxxxxkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkxkkkkkkkkkkkkkkkkkkk
kkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkxdoodxxkkkO000KKKXXXXXXXXXXXXXXKK0OkkkkkkkkxkkxxxxxxxxxxxxkkOOO00000000KKKKKKXXXXXXXXNNNNNNXXXXXXXKKKKKK0000000000000000000000000KKKKKK0000000OOOOOO0OOOOOOOOOOO00KKKK00000OOOOOkxxxxdddddxxxxdxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxkkkkkkxxkkkkkkkkkkkkkkkkxxkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkk
kkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkddddxxxkkkOO000KKXXXXXXXXXXXXXXXKKOkkkkkkkkkkkkkkkkkkkkkkxkkkOOOOOO000000000KKKKKXXXXXXXXXXXXXXKKKKKKKKKK0000000000000000000000000KK000000000OOOOOOOOOOOOOOOOOOO00KKKKK000000OOOkkxxxxxxxxxxxxxxxxxxxxxxxxxxxxxkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkxxkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkx
kkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkdddxxkkkOOO0000KKKKKXXXXXXXXXXXXXK0kkkkkkkkkkkkkkkkkkkkkxxxkkkkkkkOOOOOOOOOO000KKKKKKKKKXXXXXKKKKKKKKKKKKKKKK00000000000000000000000000000OOOOOOOOOOOOOOOOOOOOOO0KKKKKKK000000OOOkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkxxxxxxxxxxxxdd
kkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkxddxkkkOOOO0000KKKKKKXXXXXXXXXXXXK0kkkkkkkkkkkkkkkkkkkkkxxxxxkkkkkkkOOOOOOOOOOO0000KKKKKKXXXKKKKKKKKKKKKKKKKK0000000000000OO00000000000000OOOOOOOOOOOOOOOOOOOOO000KKKKKK0000000OOOkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkxxxxxxxxxxxdddooooooooooo
kkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkxddxkkOOOO00000KKKKKXXXKKXXXXXXXXXKOkkkkkkkkkkkkkkkkkkkkxddxxxkkkkkkkkkkkkkkkkOOOO00000KKKKKKKK000000000K00000000000000000O00000000000000OOOOOOOOOOOOOOOOOOO000000KKKK000000000OOOkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkxxxxxxxxxxxxxdddooooooooooooooooddddxxxx
kkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkxdxxkkOOO000000KKKKKKKKXKKXXXXXXXXK0OkkkkkkkkkkkkkkkkkkkxxxxxkkkkkkkkkkkkkkkkkkkkkOOOOOO000000000000000000000000000000000000000000000000OOOOOOOOOOOOOOOOOOO000000KKKK0000000000OOOkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkxxxxxxxxxxxxxddddoooooooooooooooooddddxxxxxxxxxxkkkkkkk
kkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkdddxkkOOO0000KKKKKKKXXXXKKKKXXXXXXK0OkkkkkkkkkkkkkkkkkkkkxxkkkkkkkkkkkkkkkkkkxxxdxxxxxxkkkOOOOOOOOOOOO00000000000000OO0000000000000000OOOOOOOOOOOOOOOOOOOO00000000KKK0000000000OOOkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkxxxxxxxxxxddddddooooooddooooooooooddddxxxxxxxxxxxxkkkkkkkkkkkkkkkkkk
kkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkdodxkkOOOOOO000KKKKXXXXXXKKKKKXXKKKK0kkkkkkkkkkkkkkkkkkkkxkkkkOOOkkkkkkkkkkkkkkxxxdddodddddxxxxkkkOOOOOO00OOOO0OOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOO00000000000000000000000OOkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkxxxxxxxxxxxxddddddddooooodddddddddddddddddddxxxxxxxxxxxxkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkk
xxxxxxxxkkkkkkkkkkkkkkkkkkkkkkkkkxdodxxkkOOOOO0000KKKXXXXXKKKKKKKKKKKK0OkkkkkkkkkkkkkkkkkkxxkkOOOOOOOOOOOkkkkkkkkkkkxxdoooooddxxxxxkkkkkOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOO000000000000000000000000OOkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkxxkxxxxxxxxxxxxddddddddoooodddddddddddddddddddddxxxxxxxxxxxxxxxkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkk
ddddddddddddddddddddxxxxxxxxxxxxxxdoddxkOOOOOO0000KKKKXXXKKKKKKKKKKKKK0OkkkkkkkkkkkkkkkkkkkkOOOO000000000OOOkkxxxxxxxxddooodddxxxxxxxxkkkkkkkkkkkkkkkkkkkkkkkOOOOOOOOOOOOOkkOOOOOOOOOOOOO00000000OOOOOOO0000000000OOOkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkxxxxkxxxxxxxxxxxxxdddddddddddddddddddddddddddddddddddxxxxxxxxxxxxxxxkxkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkk
xxxxxxxxxxxxxxxdddddddddddddddxdddooodxkOOOOOOO0000KKKKKKKKKKKKKKKKKKK0OkkkkkkkkkkkkkkkkkkkOOOO000000000000OOOkkxxxdddddddddddxxxkkxxxxxxxxkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkOOOOOOOOO0000OOOOOOOO00000000000OOOkxkkkkkkkkkkkkkkkkkkkkkkxxxxxxxxxxxxxxxxxxxxxddddddddddddddddddddddddddddddddddddddddddxxxxxxxxxxxxxxxkxxxxkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkx
kkkkkkkkkkkkkkkkkxxxxxxxxxxxxxxxxxdoodxkkOOO0OO000000KKKKKKKKKKKKKKKK00kdddddddddddddxxxxkOOO00000000000000000000OOkkxxxxxxxxxxddxkkxxxxxxxxxxxxxkkkkkxxkxxxxkkkkxxxkkkkkkkkkkkkkkkkkkkOOOOOOOOOOkkkkOO0000000000OOOkkxxxxxxxxdxxxxdddddddddddddddddddddddddddddddddddddddddddddddddxxxxxxxxxxxxxxxxxxxxxkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkxxxddddoooolloo
kkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkdooodxkOOOOOO000000KKKKKKKKKKKKKKK000kxxxxdddddddddddxkkOOO0000000000000000KKKKKK000OOOOOkkkxxddxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxkxxkkkkkkkkOOOOOOOkkkkkkOO000000000OOOkkxxdddddddddddddddddddddddddddddxxxxxxxxxxxxxxxxxxxxxxxxxxxxkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkxxxxxddooooollllllloooodddxxx
kkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkxolodxkkOOOO0000000KKKKKKKKKKKKKKK000OkkkkkkkkxkxxkxxxkkOOO0000000000000KKKKKKKKKKKKKKKK000OkxxddddxxddxxddxxxxxxddddddddxddddddxxxxxdddxxxxxxxkkkkkkkkkkkkkkkxxxkkkOOOOOOOOOOkkkkxxxxxxxxxxxxxxxxxxxxxxxxxxxxxkxkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkxxxxdddooooooolllloooooodddxxxxkkkkkkkkkkkk
kkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkxooodxxkkOOO0000000KKKKKKKKKKKKKK000OOkkkkkkkkkkkkkkkkkkOOO00000000000K0000KKKKKKKKKKKKKKKKK0Okxddddddddddddddddddddddddddddddddddddddddddddxxxxxkkkkkkkkkkkkkxxxkkxxxkkkkkkxxxxxxxxxxkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkxxxxddddooooooooooolooooooodddxxxxkkkkkkkkkkkkkkkkkkkkkkkkkk
kkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkdooodxxkOOO000000KKKKKXXXXKKKKKK000OkkkkkkkkkkkkkkkkkkkOOO0000000000K00000000KKKKKKKKKKKKKKK0Okddooooooooooodddddddddddddddddddddddddddddddddddxxxxxxkkkkkkkkxxxxxdoooooddxxxxxkkxxxkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkxxxxdddddoooooooooooooooooddddxxxxxkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkk
dxxxxxxkkkkkkkkkkkkkkkkkkkkkkkkkkkkdoooodxkkOOO0000KKKKXXXXXXXKKKKK00OOkkkkkkkkkkkkkkxkkkkOOO000000000KKKKK0000000KKKKKKKKKKKKKK0Okdooooooodoooooddoooodddddddoooooooooooooooooddddddxxxxxxxxxxxxxxxdollllodxkkkkkxxddxkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkxxxxxxddddddoooooooollooooooooddddxxxxkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkk
ddddddddddddddddddddddddxxxxxxkkkkkxoooodxxkkOOO000KKKKXXXXXXXKKKKK00OkkkkkkkkkkkkkkkxxkkOOOO000000000KKKKKK00000KKKKKKKKKKKKKKKK0Okddooooooooooooooooooooooooddoooooooooooooooooooddddddxxxxxxdddddollloodxxxxxddddddxkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkxxxxddddddoooooooooooolloooooooooddddxxxxxxkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkk
kkkkkkkkkkkkkxxxxxxxddddddddddddddddooooddxxkkOO0000KKKKXXXKKKKKKKK00OkkkkkkkkkkkkkkkxxkkOOOO0000000000KKK0000KKKKKXXKKKKK000KKKKK00kxdoooooooooooooooolooooodkOOOOOkkxoooooooooooooooodddddddddddoolllodddddddddxxxkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkxxxxxxxxxdddddddddooooooooooooooooooooooooooodddddxxxxxxkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkk
kkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkdoooodxxkkkOO0000KKKKKKKKKKKKKK0OxddddddddddddddddxkkkOOOO0000000000000000KKKKXXXXXXXXKKKKKKKKK00OxooooooolllllllllloodxkkxO00000kkxdoooooooooooloooooooooooooooodxkkkkkkkkkkkOOOOkxxxdddddddddddddoooooooooooooooooooooooooooooooooddddddddxxxxxxkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkk
kkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkxoooodxxxkkkOOO000KKKKKKKKKKKKK00kkkkkkxxxxxxxxxxdxxkkOOOOO0000000000000000KKKKXXXXXXXXKKKKKKK0000kxoloollllllllllooodxO00kxdodddkkO0Okxolooooollllllllloooooooxk000000O00000000O0Oxdoooooooddddddddddddxxxxxxxxkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkk
kkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkdlooddxxxkkkkOO00KKKKKKKKKKKK00OkkkkkkkkkkkkkkOkxxkkkOOOOO000000000000000KKKKKKKKKKKKKXXKXXKK0000OkxolllllllllloodxkxdxkOkdlcccldkO0OOOxdoooollllllclllooooodxO000000000000KKK000OOkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkk
kkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkxoloddxxkkkkkkOO000KKKKKKKKKK00OkkkkkkkkkkkkkkkkxxkkkOOOOOOO000000000000KK00KKKKKKKKKKKKKXXXXK0OkkOkxdlccccloddxxdoodxxk0KKKkddO0KK0OkxxxddoolllllllllllloodxO0000000000KKKKKK0000Okkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkk
kkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkdloodxxkkkkkkOOO00KKKKKKKKKK00OkkkkkkkkkkkkkkkkkkkkkkOOOOOO000000000000000KKKKKKKKK0KKKKKXXXXK0kxkkkkxolcclodxkkkkkkkO0OkkkkxxO000kdodkkkxoollllllllllloodxO0000KKKKKKKKKKKKKK000OOkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkk
kkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkxooodxkkkkkOOOOOO000KKKKKKKK0OOkkkkkkkkkkkkkkkkkkOOkkkkOOO0000000000000000KKKKKKKKKK000KKKKKXXXKkddxkxxdlclllll::ododxxdolllllodddxddxdodooollllllllllloxkOO000KKKKKKKKKKKKKKK0000Okkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkk
kkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkdlllodkkkOOOOOOOO000KKKKKKK0OOkkkkkkkkkkkkkkkkkOOOOkkkOOO0000000000000000KKKKKKKKKK00KKKKKKKKXXKOxddddddoloooolloolllcccccccclllloocloodxdollllllllodxkOOO0000KKKKKKKKKKKKKKK0000Okkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkk
kkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkoloodkkOOO00000000KKKKKKKK0OkkkkkkkkkkkkkkkkkOOOOOkkkOOO0000000000000000000KKKKKKKKKKKKKKKKKXXXXKOdooooolclloolc:ccccc::;;;::ccccllooolollccclllodxkOOO00000KKKKKKKKKKKKKKKKK000Okkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkk
kkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkOxloodxkO000KKKK00KKKKKKKXK0OkkkkkkkkkkkkkkkkkOOOOOkkkOOOO000000000000000000KKKKKKKKKKKKKKKKKKKXKKK0kolllccccccc:::cc:;,,,,,,,:ccclcccccclcllodxxkOOOO0000KKKKKKKKKKKKKKKKKKKK000OOkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkk
kkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkOkdoodxkOO00KKKKKKKKKKKXXXK0OkkkkkkkkkkkkkkkkkkkkkkkkkkOOOO0000000000000000000KKKKKKKKKKKKKKKKKKKKKKKOdlccccccccc:::;;,;loxxo:;::ccccc::clodxkkOkkkOO000KKKXXXKKKKKKKKKKKKKKK0000OOkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkxx
kkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkxoodxxkO00KKKKKKKKKKKKXXXKOkkkkkkkkkkkkkxxkkkkkkkkkkkkOOOO0000000000000000000KKKKKKKKKKKKKKKKKKKKKK00xl:ccccccccc:::ldOKXXK0xlc:cccc:cldxkkkkkkkkOO00KXXXXXXXXXXXKKKKKKKKK00000OOkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkxxxxxxxxxxxxxdd
kkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkdodkkkO00KKKKKKKKKKKKXXXK0kkkkkkkkkkkkkxxxkkkkkkkkkkkkOOOO0000000000000000000KKKKKKKKKKKKKKKKKKKKKKK0kl::ccccccccclokOKXXXXXOocccccloddxxxddddkO0KKXXXXNNXXXXXXXXXXXXXKKKK0000OOkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkxxxxxxxxxxxxxxxddddddddddooooo
kkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkxodkkkO00KKKKKKKKKKKKKXXXKOkkkkkkkkkkkxddxxxxkxxxxxkkkkOOOOOO00000O000000000000KKKKKKKKK00KKKKKKKKKKK0ko:;:c::clldooxO0KXXXKKkllccloodooooodkOKXXXNXXXNNNNNNXXNXXXXXXXXKKKK000OOkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkxxxxxxxxxxxxxxxxdddddddddddddoooollllllllllooll
kkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkdoxkkOO000KKKKKXXXKKKKXXX0OkkkkkkkkkkxdddxxxxxxxxxxxkkkOOOOOOOOOOOO000OOO00000KK00KKKKKK00KKKKKKKKKKK0Oo;,;:::ldOxldkO0XXXKKOxolllooolodxOKKKXXXNNNNNNNNNNNNNNNXXXXXXXKKKK0000OOkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkxxxxxxxxxxxxxxxxxxxddddddddddoodoooolllllllllllloollloooooodddddd
kkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkOkdoxkOOO00OO0KKKKKKKKK0KKXX0OkkkkkkkkkdoodddxxxxxxxxxkkkOOOOOOOOOOO00OOOOO0000000000KKKK0000K000KKKKKK00Oo;',;:okKOllxO0KKXXK00kllolloxO0KKKKXXXXXXNNNNNNNNNNNNNNNXXXXXXKKKK000OOkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkddddddddddddddddddooooolllllllllllllloooooooooddddddddddddoooooooolo
xxxxxxxxxxxxxxxxxxkkkkkkkkkkkOOkkkkkkkOOkkkxddkkkOOOOOOOO0000000000KKKOkkkkkkkkkdooodddddddddxxxkkkkOOOOOOOOO0OOOOO000000000000KKK000000000KKKKK000ko,..:x0XKdlodxk0KK0OO0klloxO0KKKKKXXXXXXXNNNNNNNNNNNNNNNXXXXXXXKKKK00OOkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkxxxxxxxxxxxxkkkkkkkkkkkkkkkkkkkkkkkkkkkkkxkxoooolllllllllloooolooooooooodddddddddddooooooooooooooooooooddddxx
ddddddddddddddddxdxxxxxxxxxxxxxxxxxxxxxxxxxxxdddxxkkkxxxxkkkOOOOkkOO0K0OOOkkkkOkdlooooddddddddxxxkkkkOOOOOOOOOOOOOO000000000O00000000000K000000K000Oko,'d0KKKKkolloxxxkxOXKxxO000KKKKKXXXXXXXNNNNNNNNNNNNNNNXXXXXXXKKKK00OOkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkxxxxxxxxxxxxxxxxxxxxxxxxxxxxddddddddddxxkkkkkkkkkkkkkkkkkkkkkkkkkkkkxxkkdoooloooooddddddddddooooooooooooooooooooooodddddxxxkkkkkkkkkkkkkk
lllllllllllllllloooooooodddddddddddddddddddxxxdoooddddddddddxxkOOOOkO00OkxxxxxxxolloooddddddddxxxxkkkkkkkkOOOOOOOOO0000000O0000000000000000000000000OkooO00KKX0o;::ccccoOXX0000KKKKKKKXXXXXNNNNNNNXXXNNNNNNNNXXXXXXKKKKK00Okkkxxkxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxddddddddddddddddddddddddddoooooolllllllldkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkxkkkkkxxdoooooooooooooooooooodddddxxxxxkkkkkkkkkkkkkkkkkkkkkkkkkkkkk
xxdddddddddoooooooooolooolllllllllllllllllloodoollooooddddodddxxkkOOkO00OxddddddollllooodddddddddxxkkkkkkkkkOOOOOOOOOO000O000000000000000000000000000OkkkkkOO0Odc,'''';dOKK000KKKKKKXXXXXXXNNNNNNXXXXXXNNNNNNXXXXXXKKKKKK0Okxxxxdddddddddddddddddddddddddddddddddoooooooollllllllllllllllllllllllloolloooooddxkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkxkkkkkxxdooodddddxxxxkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkk
oooooooodooodddddddddxxxxxxxxxxxxddddddoooolccccccllooodddddxxxxxxkkkkO0KOxdollllllllooodddddddddxxxxkkkkkkkkkkOOOOOOOOOOOOOOO00000000000000000000000Oxxddodddddo:;,,:lxkOOOO0KKKXXXXXXXXXXXNNNNNXXXXXXXXXXNNXXXXXXXXXKKK00koooooooollllllllllllllllllllllllllllollloooolloooooooooodddddddddddddddddddddxkkxxkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkxxkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkk
xxxxxxxxddddddddddoodddddooooooooooooooooool:::::cccllloddddxxkkkkkkOOOO0KXKK0kxollllooodddddxddxxxxxxkkkkkkkkkkOOOOOOOOOOOOOO0000000000000000000000Oxooolcccclll:;;;clooddddk0KKXXXXXXXXXXXXNNNNXXXXXXXXXXXNXXXXXXXXXXKKK0Odoooooooooooooooddoddddddddddddxxddddddddddddoooooooooooooooooddddddddddddddddxkxxkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkxkkkkkxxkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkk
kkkkkkkkkkkkkkkkkkkkkkkkkkxxxxxxxdddddddddddoc::::::::cclooooodxkkOOO00O00KKKXXK0xollloooddxxxxxxxxxxxkkkxkkkkkkkOOOOOOOOOOOOOO00000000000OOOO00000OOxccllc:ccccc:::::ccccllok0KKKXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXKKK0Oxdddddddddoodoooooooooooddoodddddoooodddoddddddddddddddxxxxxxkkkkkkkkkkkkkkkxkkxxkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkxkkkkkxxkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkk
kkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkoc::;;;;;:::cllcccodxkkOO00000KKKKKXK0xoloooddxxxxxxxxxxxkkkkkkkkkkkkkkOOOOOOOOOOOO0000000000OOOOOOOOOOOxc,;c:::cccc:::::::ccclokO0KKKKKXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXKKK0kdxdddddddddddddxxdxxxxxxxxxkkkkkkkkkkkkkkkkkOOkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkxkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkxkkkkkxxkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkk
kkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkxc::;;:;;;:::clllcclodxkOOOO000KKKKKKKKOdoooddxxxxxkkkkkkkkkkkkkkkkkkkkkkkkkOOkOOOOO000000000OOOOOOOOOOkxl,',:::::::::::::ccccloxO0KKKKKKXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXKKKK0OOOOOOkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkxkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkxkkkkkxxkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkk
kkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkl:::;;;;;::::ccloollodxxkkOO000KKKKKKKKKkdoddxxxxkkkkkkkkkkkkkkkkkkkkkkkkkkkkkOOOOOO00000000OOOkkkOOkkkxo:'.....''',;:::ccc::cdxO0KKKKKKKXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXKKKK0Okkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkxkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkxxkkkkxxkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkk
kkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkdc::;;;;;;;;:::ccllcclodxxkOO0000000K00KK0kdddxxkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkOOOOOO000000000OOkkkkkkkkxoc;.     ...',,;,;,,:oxkO0000KKKKKKKXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXKKKKKOOkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkxxkkkkkkkkkkkkkkkkkkkkkkkkkkkkkxxkkkkxxkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkxxkkkkkkk
kkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkdc::;;;;;;;;;;:::ccclllodxxxkOOOOO0000000KK0xdxxxkkkkkkkkkkkkkkkkkkkkkkkkkkkkkOOOOOO000000000OOOOkkkkkkxoc;'     ...'''',,,:odxkOO0000KKKKKKKKXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXKKXKKKOkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkxkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkxkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkk
kkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkdc::;;;;;;;;;;::::::cllooddxxxkkOOO000OO00KKKOxxxkkxkkkkkkkkkkkkkkkkkkkkkkkkkkOOOOOOO000000000OOOkkkkkxdol:,.   ..'',,,,;;:ldxkkOO000KKKKKKKKKKKKXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXKKK0OkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkxkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkxkkkkOkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkk
kkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkdcc::;;;;;;;;;;:::::::looodddddxkOOO0OO0OO00O0OxxxxxkkkkkkkkkkxxkkkkkkkkkkkkkkkkkOOOOOOOO000000OOkkkkxxdol:,.   ..',,,;;;:lddxxkOO0000KKKKKKKKKKKKXXXKXXXXXXXXXXXXXXXXXXXXXXXXXKKKK0OkkOkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkxkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkxxkkkOkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkk
kkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkxlc::;;;;;;;;;;;:::::::cloooooodxkOOOO0OkkkkkOOkxxxxkkkkkkkkkkxxxkkkxxxkkkkkkkkkkkOOOOOOOOO000OOOkkkxxddolc;.   ..',,;:::lodxxxkOOO00000000KKKKKKKKKKKKKKKKKKKKKKKXXXXXXXXXXXXXXXXXK0Okkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkxkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkxkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkk
kkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkxoc::;;;;;;;;;;;:::::::::llooolloxxkkOOdloxxkO0OkxxkkkkkkkkkkxxxxxxxxxxxxxxxkkkkkkOOOOOOOOOO00OOOkxxxxddolc:'  ..'',;;::clodxxxkkOOO0000000KKKKKKKKKKKKKKKKKKKKKKKKXXXXXXXXXXXXXXXXKK0OkkkkkkkkkkkOOkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkxkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkxkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkk
kkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkdl:::;;;;;;;;;;;;::::::;:clddddooddxOOdcldxxkO0OkkkkkkkkkkkkxxxxxxxxxxxxxxxxkkkkkOOkkOOOOOOO00OOkxxxdddolc:'   .'',;;:cloddxxxkkOOOO0000000K00KKK00KKKKKKKKKKKKKKKXXXXXXXXXXXXXXXXKKK0OkkkkkkkkkkkkkkkkkOkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkxxkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkk
kkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkoc::;;;;;;;;;;;;;;::cccccloddoooddxkOkolodxkO00OkkkkOkkkkkkxxxxxxxxxxxxxxxxxxkkkkkkkkOOOOOO0OOOkkxddddolc:,. ..',;;:cllodddxxkkkOOOO0000000000000000KKKKKKKKKKKKKXXXXXXXXXXXXXXXXXXKK0OkkkkkkkkkkkkOOkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkxxkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkxkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkk
kkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkxoc::;;;;;;;;;;;;:c::ccc::::cllodxkOO0kocodxkO00OkkkkkkkkxxxxxxxxxxxxxxxxxxxkkkkkkkkkOOOOOOOOOOOkxxddoolc:,.  .',;;:cllodddxxxkkkOOOO000000000000000KKKKKKKKKKKKXXXXXXXXXXXXXXXXXXXXK0OOkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkxxkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkxkkkOkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkk
kkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkxl:::;;;;;;;;;;;;::;;;;;;;::ldkkkkOKKKOocldxkO00OkkkkkkxxxxxxxxxxxxxxxxxxxxkkkkkkkkOOOOOOOOOOOOkxxddoolc:,.  .',;:ccllloddddxxkkkOOOOO00000000000000KKKKKKKKKKKKKKXXXXXXXXXXXXXXXXXKK0OkkkkkkkOOkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkxxkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkOkxkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkk
kkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkdc:::;;;;;;;;;;;::::::::::::ldkOOOO000OocdkkO000OkxxxxxxxxxxxxxxxxxxxxxxxxxxxkkkkOOOOOOOOOOOOOkxxddoolc:,.  ..,;:cclllodddxxxxxkOOOOOOO000000000000KKKKKKKKKKKKKKKKXXXXXXXXXXXXXXXKKK0OOOkkkkOOkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkxxkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkOkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkk
kkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkOkkkkkkkkkkkkkkOko:::;;;;;;;;;;;:cccccc::::::coxxxxkkO0xcokkxxkkkkxxxxxxxxddxxxdxxxxxxxkkxxxxxkkkkOOOOOOOOOOOkkxxddoolc:,.  ..';::cllloodddxxxxkkkOOOOOOOOOO00000000KKKKKKKKKKKKKKKKXXXXXXXXXXXXXXKKK0OOOOkkkOOkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkxxkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkOkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkk
kkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkdc::::::;;;;;;;;:lloolc:;::;;:loxxxkkOkccoxxolodxxxdddddddxxxxxxxxxxxxxkkkkkkkkkkkkkOOOOOOOOkkxxdoooll:;.  ..';::clllloodddxxxxkkkkkOOOOOOOOO0000000000KKKKKKKKKKKKKXXXXXXXXXXXXXXKKK0OOkkkkkOOkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkxxkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkOkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkk
kkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkOkkkdc::::::;;;;;;;;:cccclc;;;;;;;:ldxxkOOkl:loxxxxxxxxdddddddxxxxddddxxxxxxkkkkkkkkkkkkkkkOOkkkkkxxdoollc:;.  ..,;::ccllloooddddxxxxkkkkkkOOOOOOO0000000000KKKKKKKKKKKKKKKXXXXXXXXXXXXKK00OkkOkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkOkkkkxxkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkOkkkkOOkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkk
kkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkdc::::::;;;;;;;;;:::::;;;,,,;:coxkOO0Oo;:lodxddxxxddddddddxxdddddxxxxxxxkkkkkkkkkkkkkkkkkkkkxxddoollc:;.  ..';::cccllloodddddxxxxxkkkkkOOOOOO0000000000000000KKKKKKKKKKKKXXXXXXXXXKKK0OOOOkOOkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkOkkkkxkkkkkkkkkkkkkkkkkkkkkkkkkkOkkkkkkkkOkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkk
kkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkxdolc::::;;;;;;;;;;;;;;,'..,;:oxOO00KOl;:codxkOOxdxdddddddddddddxxxxxxxkkkkkkkkkkkkkkkkkkkkxxddoollc:;'....',::cccllllooodddddxxxxxkkkkkkkOOOOO0000000000000KKKKKKKKKKKKKKKKXXXXXKKK00OOOkOOkkkkkkkkkkkkkkkkkkkkkkkkkkOkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkxxkkkkkkkOOkkkkkkkkkkkkkkkkOOkkkxkkkOkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkk
kkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkxoc:::;;;;;;;;;:::;;'..';cldkOO00Kx:;ccoxkOOkxdddddddddddddddxxxxxxxxxkkkkkkkkkkkkkkxxxxxddoolcc::;'...',;:ccclllllooodddddxxxxxxxxxkkkOOOOO0000000000000KKKKKKKKKKKKXXKKXKKKKKKK0OOkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkOkkkkxxkkkkkkkkkkkkkkkkkkkkkkkOOOkkkkkkkkOkkkkkOkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkk
kkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkOkxdolc:::;;;:::ccc:,...,:codxOO000l,;:codxxdolodddddddddddddddxxxxxxxxkxxxxxkkkkkkkxxxxxddollcc:coc''',,;::cclllllooooodddddxxxxxxxxkkkkOOO00OOOOO0000000KKKKKKKKKKXXXKKXKKKKKKK00OOOOOkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkOkkkkkxkkkkkkkkkkkkkkkkkkkkkkkkkkkkOkkkkkOkkkkkOkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkk
kkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkxl:::;;::::cc:,....;:cldkkO00x;',,;:c:;:clooooooddddddddddxxxxxxxxxxxkkkkkkkkxxxxxxdoollcc:cxkl,,,,;:::ccccllllooooddddddxxxxxxxkkkkOOOOOOOOO00000000KKKKKKKKKXXKKXXXKKKKK000OOOkOOkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkOkkkkkxkkkkkkkkOOkkkkkkkkkkkkkkkkkkkkxkkkOkkkkOOkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkk
kkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkxl:::;:::::c:;,'..';:clodkOOOd:;'....';clooooooddddddddddxxxxxxxxxxxkkkkkkkxxxxxxxddoolcc:lkOxl;,;;:::ccccllllllooooddddddxxxxxkkkkOOOOOOO00000000000KKKKKKKKKKKKKKKKKKKKK000OOkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkxxkkkkkkkkkkkkkkkkkkkkkkkOOOkkkkkkkkOkkkkOOkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkk
kkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkOxoc::::::::::;,..',;:clldxkkOOOkdl,',:clooooooooodddddddxxxxxxxxxxxxkkkkxxxxxxxxxddoolc::dOkkxc;;;:::cccccllllloooooodddddxxxxxkkkkOOOOOOOOO000000000KKKKKKKKKKKKKKKKKKKK000OOkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkOkkkkkxkOkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkOkkkkOOkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkk
kkkkkkkkkkkkkkOkkkkkkkkkkkkkkkkkkkkkkkkOkkkkkkkkkkkkkkkkkkkkkkkkkkkdl:::::;:::;,''.,;;:codxkkxkOO00x:,:cllloooooooooooddddxxxxxxxxxxxkkkxxxxxxxxxdddolcc;cxOkkkxc;;::::cccccclllllloooodddddxxxxxkkkkOOOOOOOOO00000000000KKKKKKKKKKKKKKKKKK000OOkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkOkkkkkxkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkOOkkkOOkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkk
kkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkOOkkkkkkkkkkkkkkkkkkkkkkkkkkkkocc::::::;;;,,;:::clooodooodddl;,;:cllloooooooooddddddddddxxxxxxxkkxxxxxxxxxxddooc::okkkkkkxl::::ccccccccccllllooooodddddxxxkkkOOOOOOOOOO0000000000KKKKKKKKKKKKKKKKKKKK000OkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkOOkkkkkxkkkkkOkkkkkkkOkkkkkkkkkkOOOkkkkkkkkOkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkk
kkkkkkkkkkkkkkkkkkkkkkOOOkkkkkkkkkkkkkkkOOOkkkkkOkkkkkkkkkkkkkkkkkkkOkkxxdl::::;;;,;cc:;;;::::::::;;,,;::cclllooooooooodddddddddxxxxxxxxxxxxxxxxxxxddoc:cxkkkkkkkxl:::ccccccccccclllloooooddddxxxxkkOOOkkkOOOO00000000000KKKKKKKKKKKKKKKKKKKK00OOkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkOkkkkkkkkkkkkkkOkkkkkxkkkkOkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkOkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkk
kkkkkkkkOkkkkkOkkkkkkkOOkkkkkOkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkxl:::::;;;:c::;;;;;;;,'.'',;;::ccllllloollooooooooodddddxxxxxxxxxxxxxxxxxxdoc:okkkkkkkkOxl:ccccccccccccllllllloooooddxxxkkOOOkkOOOOOOOO00000000KKKKKKKKKKKKKKKKKKK000OOkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkOkkkkkxkkkOOkkkkkkkkkkkkkkkkkkkOkkkkkkkkkOOkkkkOkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkk
kkkkkkkkkkkkkkkOkkkkkOOOOkkkkOkkkkkkkkkkOOOkkkkOOOkkkkkkkkkkkkkkkkkkkkkkkkkdl::::::::::;;,,,,,,'...',;;;:ccclllllllllooooooooodddddddxxxxxxxxxxxxkxxdoccdOkkkkkkkkkxlcccccccccccccllllllooooodddxxkkkOOOOOOOOOOOO00000000000KKKKKKKKKKKKKKK0000OOOOkkkkkkkkkkkkkkkkkkkkkkkkOkkkkkkkkkkkkkkkkkkkkkkkkkkkkOkkkkkxkkkOOkkkkkOkkkkkkkkkkkkkkOOkkkkkkkkOkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkk
kkkkkOOOOOkkkOOOOkkkkOOOOkkkOOOkkOOkkkkkOOkkkkkkkkOOkkkkOkkkkkkkkkOOOkkkkOkkkxdoc:::::::::;;;;;;,,,,;;;;::ccllllllllllllooooooodddddddddxxxddxxxkkkxxoclkOkkkkkkkkkOkoccccccccccccclllllllloodddxxkkkOOOOOOOO0OO0000000000000KKKKKKKKKKKKKK00000OOOkkkkkkkkkkkkkkkkkOkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkOkkkkkxkkkkkkkkkkOkkkkkkkkkOOkkkkkkkkkkkkkOkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkk
OkkkkkOOOkkkkOOkkkkkkkOOkkkOOOOOkkkkkkkkOkkkkkkOOOOOOOOOOOOOOkkkOkkOOkkkkkkkkkOkkxdolc:::cllllc::;;;::;;:::cccccllllllllooooooooddddddddxxxxxxxxkkkkxocdkkkkkkkOkkOkkkdcccccc::ccccccllllllooooddxxkkkOOOOOOO0000000000000000KKKKKKKKKKKKKK00000OOOkkOOOOOkkkkOkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkOkkkkkxkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkOkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkk
kkkkOOOkkkkkkkkkkkkkkkkkkkOOOOOOOOkkkkkkOOOOOkkkkkkOOOOkkkkkOOOOkkkkOOkkkkkkkkkkkkkkkxxxxkkkkxoc:::::::;:::ccccccllllllllooooooooooddddddddxxxxkkkkkkdlxOkkkkkkkkkOkkkkxlcccccc:ccccccclllllooooddxxkkkkOOOOO00000000000000000KKKKKKKKKKKK0000000OOkOOkkkkkkkOOOkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkOkkkkkxkkkkkkkOkkkkkkkkkkkkkkkkkkkkkkkkkkkOkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkk
kkkkkkkkkkOkkkkkkkkkkkkkkkkkkkkOOOOOOkkkOOOOOOOkkkkOOOOkkkkkkkOkkkkkOOOkkkkkOOOkkkkkkkOkkOkkkkdlc::c:::;:::::cccclllllllllllllloooooodddddddxxxkkkkkkdoxOkkkkkkkkkkkkkOkxocccccccccccccclllloooooddxxkkkkkOOO0000000000000000KKK00KKKKKKKKK000000OOOOOkkkOkkkkkOkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkOkkkkkxkkkkkkkOOkkkkkkkkkkkkkkkkkkkkkkkkkkOkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkk
kkkkkkkOOOkkkkkkkkkkkkkkkkkkkkkkOOkkkkkkkkOOOOOOkkkOOOkkkkkkkkkkOkkkOkkkkkkkkkkkkkkkOkkkOkkkkkxlccc:::::::::cccccclllllllllllllllloooooodddddxxkkkkkkxdkkkkkkkkkkkkkOOOkkkdlccccccccccccccllllloooddxxkkkkkOOOO00000000000KKKKK0KKKKKKKKKKK0000000OOkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkOOkkkkkxkkkkkkkOOkkkkkkkkkkkOOOkkkkkkkkkkkkOkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkk
kkkkkkkkkkkkkkkkkkkkkkOkkkkkkkkkkkkkkkkkkkkOOOkkkkkkkkkkkkkkkkkkkkkOOkkkkkkkkkkkkkkkkkkkkkkkkkkdccc:::::::::cccccccclllclllllllllllooooodddddxxxkkkkkxxkkkkkkkkkkkkkkkOkkkkxoccccccccccccccllllloooddxxkkkkOOOOOO00000000K0000K0KKKKKKKK0000000000OOkkkkkkkkkkkkkkkkkkkkkkkkkOOOkkkkkkkkkkkkkkkkkkkkkkkkOkkkkkxkkkkkkkOkkkkOkkkkkkkkkkkkkkkkkkkkkkOkkkkOkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkk
kkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkOOOkkOkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkxocc::::::::::ccccccccccccllllllllllllooooooddxxxkkkkkkkkkkkkkkkkkkkkkkOkkkkkxolclcccccccccccllllloooddxxxkkkkkOOO0000000000KKKKKKKKKKK000000000000OOOkkkkkkkkkkkkkkkkkkkkkkOOOOOkkkkkkkkkkOOkkkkkkkkkkkkkkkkkkxkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkOkkkkOkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkk
kkkkkkkkkkkkkkkkkkkkOkkkkkkkkkkkkkkkkkkkOOOkkkOOkkkkkkkkkkkkkkkkOkkkkkkOOkkkkkkkkkkkkkkkkkkkkkkkdlcc::::::::::cccccccccccccccclllllllllooooddxxxkkkkkxxkkkkkkkkkkkkkkkOkkkkkxkxocccccccccccccllllllooddxxxkkkkkOO000000000KKKKKKKKKK000000000000000OOOkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkxkkkkkkkOOkkkkkkkkkkkkkkkkkkkkkkkkkkOkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkk
kOOkkkOkkkkkkkkkkkkkkkkkkkkkkkkkOkkOOOkkOkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkOOOkkkkkkkkkkkkkkkkkkkkkkkoccc:::::::::cc::ccccccccccccccccllllllloooddxxxxxkkxxkkkOkkkkkkkkOkkOkkkkkxkkxdllccccccc:ccclllllloodddxxkkkkOOO00000000KKKKKKKKK0000000000000000OOOkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkxkkkkkkkkkkkkkkkkkkkkkkOkkkkkkkkkkkkOkkkkOkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkk
kkkkkkkkkkkkkkkkkkkkkkkkOOkkkkkOOkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkOkkOkkkkkkkkkkkkkkkkkkkkkkkkdcccc::::::::ccc:cccccccccccccccccclllllloodddxxxxkkxxxkkkOOkkkOkOOOOOkkkkkxkkkkxolccccccccccccclllloodddxxkkkOOOO000000000KKKKK000000000000000000OOOkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkOkkkkkxkkkkOkkOOkkkkkkkkkkkkkkkkkkkkkkkkkkOkkkkOkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkk
kkkkkkkkkkkkkkkkkkkkkkOOOOOOOOOkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkOkkkkkkOkkOkkkkkkkkkkkkkkkkkkkkOkocccc::::::::::::::::::::ccccccccccclllloooddxxxxkkxxxkkkkkkkkkkkkOOOkxkkkxkkxkOkxolcccccccccccclllooodddxxkkkOOOO00000000KKKKKKKK00000000000000OOOOkkkkkkkkkOkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkxkkkkOkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkOkkkkOkOkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkk
kkkkkkkkkkkkkkkOOkkkkkkkOOOOOOOkkkkkkkkkkkkkkkkkOOkkkkkkkkkkkOOOkkkkkkkkOOkkkkkkkkkkkkkkkkkkkkkkkkxlcccc::::::::::::::::::::::::c::ccccllloooddddxxxxxxxkkkkkkkkkkkkkkOkkkkkxkkkkkOOkdlcccccccccccclllooooddxxkkOOOO00000000KKKKKKKK00000000000000OOOkkOkkkkkkkOOkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkxkkkkOkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkOkkkkOOOOkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkk
kkkkkkkkkkkkkOOOkkkkkkkOOOOOOOOOOkkkkkkkkkkkkkkkkkOOOOOkkkkOOOOOkkkOkkkOkkkkkkkkkkkkkkkkkkkkkkkkkOkoccccc::::::::::::::::::::::::::ccccllllooddddxxxxxdxxkkkkkkkkkkkkOOkkkkkxkkkkkOOOOxllcccccccccccllloooddxxkkOOOO0000000000000KKK00000000OOOOOOOOkkkOkkkkkkkOOOkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkOOkkkkkkxkkkkkkkkkkOkkkkkOkkkkkkkkkkkkkkkkkkOkkkkOkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkk
*/
