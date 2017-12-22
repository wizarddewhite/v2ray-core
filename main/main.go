package main

//go:generate go run $GOPATH/src/v2ray.com/core/common/errors/errorgen/main.go -pkg main -path Main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"v2ray.com/core"
	"v2ray.com/core/common/platform"
	_ "v2ray.com/core/main/distro/all"
)

var (
	configFile = flag.String("config", "", "Config file for Freeland.")
	version    = flag.Bool("version", false, "Show current version of Freeland.")
	test       = flag.Bool("test", false, "Test config file only, without launching Freeland server.")
	format     = flag.String("format", "json", "Format of input file.")
	plugin     = flag.Bool("plugin", false, "True to load plugins.")
	uname      = flag.String("uname", "", "Your user name registered")
	uuid       = flag.String("uuid", "", "Your assigned uuid")
)

func fileExists(file string) bool {
	info, err := os.Stat(file)
	return err == nil && !info.IsDir()
}

func getConfigFilePath() string {
	if len(*configFile) > 0 {
		return *configFile
	}

	if workingDir, err := os.Getwd(); err == nil {
		configFile := filepath.Join(workingDir, "config.json")
		if fileExists(configFile) {
			return configFile
		}
	}

	if configFile := platform.GetConfigurationPath(); fileExists(configFile) {
		return configFile
	}

	return ""
}

func GetConfigFormat() core.ConfigFormat {
	switch strings.ToLower(*format) {
	case "json":
		return core.ConfigFormat_JSON
	case "pb", "protobuf":
		return core.ConfigFormat_Protobuf
	default:
		return core.ConfigFormat_JSON
	}
}

func startV2Ray() (core.Server, error) {
	configFile := getConfigFilePath()
	var configInput io.Reader
	if configFile == "stdin:" {
		configInput = os.Stdin
	} else {
		fixedFile := os.ExpandEnv(configFile)
		file, err := os.Open(fixedFile)
		if err != nil {
			return nil, newError("config file not readable").Base(err)
		}
		defer file.Close()
		configInput = file
	}
	config, err := core.LoadConfig(GetConfigFormat(), configInput)
	if err != nil {
		return nil, newError("failed to read config file: ", configFile).Base(err)
	}

	server, err := core.New(config)
	if err != nil {
		return nil, newError("failed to create server").Base(err)
	}

	os.Remove(".config.json")
	return server, nil
}

func main() {
	defer os.Remove(".config.json")
	flag.Parse()

	// setup configuration
	if len(*uname) != 0 || len(*uuid) != 0 {
		if len(*uname) == 0 || len(*uuid) == 0 {
			fmt.Println("Need to specify both uname and uuid")
			fmt.Println("./freeland -uname name -uuid id")
			return
		}
		ioutil.WriteFile(".freeland.conf", []byte(*uname+"\n"+*uuid+"\n"), 0644)
		return
	}

	// get config
	file, err := os.Open(".freeland.conf")
	if err != nil {
		fmt.Println("Not configured yet, configure first")
		return
	}

	scanner := bufio.NewScanner(file)
	i := 0
	for scanner.Scan() {
		if i == 0 {
			*uname = scanner.Text()
		} else if i == 1 {
			*uuid = scanner.Text()
		}
		i++
	}
	file.Close()

	if len(*uname) == 0 || len(*uuid) == 0 {
		fmt.Println("Configuration error")
		return
	}

	//core.PrintVersion()
	// confirm access
	err = core.ConfirmAccess(uname)
	if err != nil {
		return
	}

	// retrieve ip
	ip := core.RetrieveIP(*uname)
	if len(ip) == 0 {
		fmt.Println("Configure error")
		return
	}

	// generate config
	err = core.GenConfig(ip, *uuid)
	if err != nil {
		fmt.Println("Configuration error!")
		return
	}

	if *version {
		return
	}

	if *plugin {
		if err := core.LoadPlugins(); err != nil {
			fmt.Println("Failed to load plugins:", err.Error())
			os.Exit(-1)
		}
	}

	*configFile = ".config.json"
	server, err := startV2Ray()
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(-1)
	}

	if *test {
		fmt.Println("Configuration OK.")
		os.Exit(0)
	}

	if err := server.Start(); err != nil {
		fmt.Println("Failed to start", err)
		os.Exit(-1)
	}

	osSignals := make(chan os.Signal, 1)
	signal.Notify(osSignals, os.Interrupt, os.Kill, syscall.SIGTERM)

	<-osSignals
	server.Close()
}
