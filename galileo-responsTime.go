package main

import (
	"fmt"
//	"reflect"
	"sync"
	"strings"
	"net/http"
//	"encoding/json"
	"os"
	"bufio"
	"io"
	"encoding/json"
	"strconv"
	"time"
    "gopkg.in/yaml.v2"
	"io/ioutil"
	"github.com/hpcloud/tail"

	"sort"
)


type conf struct {
	VisitUrl  	string 		`yaml:"visitUrl"`
}


func (c *conf) getConf() *conf {
	yamlFile ,err := ioutil.ReadFile("responsTime.yaml")
	if err != nil {
		fmt.Println("yamlFile.Get err", err.Error())
	}
	err = yaml.Unmarshal(yamlFile, c)
	if err != nil {
		fmt.Println("Unmarshal: ", err.Error())
	}
	return c
}


func Post(data ,visitUrl string) {
	jsoninfo := strings.NewReader(data)
	client := &http.Client{}
	req, err := http.NewRequest("POST", visitUrl, jsoninfo)
	if err != nil {
		fmt.Println(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("token", "xxx")
	resp, err := client.Do(req)
	defer func() {
		if err := recover(); err != nil {
			fmt.Println(err)
			return
	}
		fmt.Println("Process panic done Post")
	}()
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(string(body))
	fmt.Println(resp.StatusCode)
}


var properties = make(map[string]string)

func init() {
	srcFile, err := os.OpenFile("./responsTime.properties", os.O_RDONLY, 0666)
	defer srcFile.Close()
	if err != nil {
		fmt.Println("The file not exits.")
	} else {
		srcReader := bufio.NewReader(srcFile)
		for {
			str, err := srcReader.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					break
				}
			}
			if len(strings.TrimSpace(str)) == 0 || str == "\n" {
				continue
			} else {
				fmt.Println(str)
				num++
				properties[strings.Replace(strings.Split(str, ":")[0], " ", "", -1)] = strings.Replace(strings.Split(str, ":")[1], " ", "", -1)
			}
		}
	}
}


type postData struct {
	ConfigId 		string 		`json:"ConfigId"`
	MinResponseTime float64		`json:"minResponseTime"`
	MaxResponseTime	float64		`json:"maxResponseTime"`
	AvgResponseTime	float64		`json:"avgResponseTime"`
}

var wg sync.WaitGroup
var num int = 0

type FloatSlice []float64
func (s FloatSlice) Len() int { return len(s) }
func (s FloatSlice) Swap(i, j int){ s[i], s[j] = s[j], s[i] }
func (s FloatSlice) Less(i, j int) bool { return s[i] < s[j] }

func main() {
	var config conf
    urlConfig := config.getConf()
    visitUrl := urlConfig.VisitUrl
	for {
		c := make(chan string, 1)
		wg.Add(num)
		for machineId ,logAbsPath := range properties {
			machineId = strings.TrimSpace(strings.Replace(machineId, "\n", "" ,-1))
			logAbsPath = strings.TrimSpace(strings.Replace(logAbsPath, "\n", "" ,-1))
			go tailLog(logAbsPath ,machineId ,visitUrl ,c)
		}
		wg.Wait()
		c <- "stop"
		close(c)
		fmt.Println("本次结束")
	}
}


func tailLog (logAbsPath ,machineId ,visitUrl string, done chan string) {
//	var count int
	var data postData
	var responseTimeList []float64
	var responseTime string
	var minResponseTime, avgResponseTime, maxResponseTime float64 
	t := time.NewTimer(time.Second * time.Duration(60))
	defer t.Stop()

	config := tail.Config{
		ReOpen:    true,                                 // 重新打开
		Follow:    true,                                 // 是否跟随
		Location:  &tail.SeekInfo{Offset: 0, Whence: 2}, // 从文件的哪个地方开始读
		MustExist: false,                                // 文件不存在不报错
		Poll:      true,
	}
	tails, err := tail.TailFile(logAbsPath, config)
	if err != nil {
		fmt.Println("tail file failed, err:", err)
		return
	}

	var (
		line *tail.Line
		ok   bool
	)

L:
	for {
		select {
		case <- done:
			fmt.Println("run stop signal")
			break L								// 跳出for循环
		case <- t.C:
			fmt.Println("run time out")
			break L
		default:
			break
		}

		line, ok = <-tails.Lines 	//遍历chan，读取日志内容
//		正则截取最后一位
//		存入切片
		responseTime = strings.Split(line.Text ," ")[len(strings.Split(line.Text ," "))-1]
		responsTimeFloat64 ,_ := strconv.ParseFloat(responseTime ,64)
		if responsTimeFloat64 != 0 {
			responseTimeList = append(responseTimeList ,responsTimeFloat64)
		}
		if !ok {
			fmt.Printf("tail file close reopen, filename:%s\n", tails.Filename)
			continue
		}
		fmt.Println("line:", line.Text)
	}

	sort.Sort(FloatSlice(responseTimeList))
	if len(responseTimeList) == 0 {
	    responseTimeList = append(responseTimeList ,0)
	}
	minResponseTime = responseTimeList[0]
	maxResponseTime = responseTimeList[len(responseTimeList)-1]
	sum := 0.0
	for _, add := range responseTimeList {
	    sum = sum + add
	}
	avgRes := float32(float32(sum) / float32(len(responseTimeList)))
	avgResponseTime ,_ = strconv.ParseFloat(fmt.Sprintf("%.3f" ,avgRes), 64)
	fmt.Println(avgResponseTime)
    data = postData{ConfigId: machineId, MinResponseTime: minResponseTime, MaxResponseTime: maxResponseTime, AvgResponseTime: avgResponseTime}
    dataJson, err := json.Marshal(data)
    if err != nil {
        fmt.Println("data json trans err", err.Error())
    }
    dataJsonStr := string(dataJson)
    fmt.Println(dataJsonStr)
    Post(dataJsonStr ,visitUrl)

//	<- done
    fmt.Println("000000000")
    wg.Done()
}
