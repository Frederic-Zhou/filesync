package main

import (
	gjson "github.com/bitly/go-simplejson"
	"github.com/golang/exp/fsnotify"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var tracks []string
var m sync.Mutex
var syncPath string
var targetPath string
var remoteHost string
var sshPort string
var excludePath []string
var fw *fsnotify.Watcher

func main() {

	confdata, err := ioutil.ReadFile("./filesync.conf")
	if err != nil {
		log.Fatalln(err.Error())
	}

	conf, err := gjson.NewJson(confdata)
	if err != nil {
		log.Fatalln(err.Error())
	}

	syncPath, err = conf.Get("syncpath").String()
	if err != nil {
		log.Fatalln(err.Error())
	}

	targetPath, err = conf.Get("targetpath").String()
	if err != nil {
		log.Fatalln(err.Error())
	}

	remoteHost, err = conf.Get("remotehost").String()
	if err != nil {
		log.Fatalln(err.Error())
	}

	excludePath, err = conf.Get("excludepath").StringArray()
	if err != nil {
		log.Fatalln(err.Error())
	}

	sshPort, err = conf.Get("sshport").String()
	if err != nil {
		log.Fatalln(err.Error())
	}

	log.Println("Start Watch")
	log.Println("remoteHost", remoteHost, "port", sshPort)
	log.Println("syncPath", syncPath)
	log.Println("targetPath", targetPath)
	log.Println("excludePath", excludePath)

	// 创建一个新的文件观察者
	fw, err = fsnotify.NewWatcher()
	if err != nil {
		log.Fatalln(err.Error())
	}

	//爬行出所有目录和子目录
	err = filepath.Walk(syncPath, getPathsFunc)
	if err != nil {
		log.Fatalln(err.Error())
	}

	// 开始监听跟踪列表
	go syncfileFunc()

	// 监听文件变化，并保存到tracks字典中
	for {
		select {
		case ev := <-fw.Event:
			// 不监听文件属性变化, 监听 新增、删除、修改、重命名
			if !ev.IsAttrib() {
				m.Lock()
				tracks = append(tracks, ev.String())
				m.Unlock()
			}
		case err := <-fw.Error:
			log.Println("error:", err)
		}
	}
}

//获取爬行的路径函数
func getPathsFunc(path string, info os.FileInfo, err error) error {
	if info.IsDir() {
		// 跳过已经被设为排除的目录
		for _, v := range excludePath {
			if strings.HasPrefix(path, v) {
				return err
			}
		}
		fw.Watch(path)
		log.Println("watching:", path)
	}
	return err
}

//每2秒处理一次所有监听到的文件
func syncfileFunc() {
	log.Println("Start syncFile")
	for {
		if len(tracks) > 0 {
			// 打印出当前跟踪列表，便于调试
			//log.Println("current list:", tracks)
			m.Lock()
			info := strings.Split(tracks[0], ":")
			f := strings.Trim(info[0], "\"")
			a := strings.Trim(info[1], " ")

			if a == "CREATE" || a == "MODIFY" {
				cmdFunc("scp", "-P", sshPort, "-r", f, remoteHost+":"+strings.Replace(f, syncPath, targetPath, 0))
				err := filepath.Walk(f, getPathsFunc)
				if err != nil {
					log.Println(err.Error())
				}
				log.Println("scp", "-P", sshPort, "-r", f, remoteHost+":"+strings.Replace(f, syncPath, targetPath, 0))
			} else if a == "DELETE" || a == "RENAME" {
				cmdFunc("ssh", "-p", sshPort, remoteHost, "rm", "-rf", strings.Replace(f, syncPath, targetPath, 0))
				log.Println("ssh", "-p", sshPort, remoteHost, "rm", "-rf", strings.Replace(f, syncPath, targetPath, 0))
			}

			tracks = tracks[1:]
			m.Unlock()
		} else {
			time.Sleep(time.Second * 2)
		}
	}
}

func cmdFunc(cmdstr string, args ...string) {
	cmd := exec.Command(cmdstr, args...)
	output, err := cmd.Output()
	if err != nil {
		log.Println(err.Error())
	}
	if len(output) > 0 {
		log.Println(string(output))
	}
}
