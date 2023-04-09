package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"strings"
	"time"
)

//这里我总结一下go的文件操作

//1。创建文件件
// newFile, err = os.Create("test.txt")
//     if err != nil {
//         log.Fatal(err)
//     }
//     log.Println(newFile)
//     newFile.Close()

//2。文件的移动（重名名）
//originalPath := "test.txt"
// newPath := "test2.txt"
// err := os.Rename(originalPath, newPath)
// if err != nil {
// 	log.Fatal(err)
// }

//3。检查文件的存在
// fileInfo, err := os.Stat("test.txt")   这句是查询文件的信息
//     if err != nil {
//         if os.IsNotExist(err) {
//             log.Fatal("File does not exist.")
//         }
//     }
//     log.Println("File does exist. File information:")
//     log.Println(fileInfo)

//4.复制文件
//  // 打开原始文件
//  originalFile, err := os.Open("test.txt")
//  if err != nil {
// 	 log.Fatal(err)
//  }
//  defer originalFile.Close()

//  // 创建新的文件作为目标文件
//  newFile, err := os.Create("test_copy.txt")
//  if err != nil {
// 	 log.Fatal(err)
//  }
//  defer newFile.Close()

//  // 从源中复制字节到目标文件
//  bytesWritten, err := io.Copy(newFile, originalFile)
//  if err != nil {
// 	 log.Fatal(err)
//  }
//  log.Printf("Copied %d bytes.", bytesWritten)

//  // 将文件内容flush到硬盘中
//  err = newFile.Sync()
//  if err != nil {
// 	 log.Fatal(err)
//  }

//5. 写文件
// err := ioutil.WriteFile("test.txt", []byte("Hi\n"), 0666)
//     if err != nil {
//         log.Fatal(err)
//     }
var targetFile string
var targetDir string
var filelist []string

var workerCount = 0     //现在有的协程数
var maxWorkerCount = 24 //最大的协程数，我要使用递归调用的每次递归我就会开一个协程，但是我又不希望协程太多，这样同样会影响性能

var searchRequest = make(chan string, 2) //channel通信用来同步各个goroutine
var workDone = make(chan bool)           //线程结束的通知，当一个线程完成了需要向这个通道喊话，
var foundMatch = make(chan string, 2)    //传递文件路径，线程找到了需要向这个通道传递文件路径

func init() {
	flag.StringVar(&targetFile, "f", "file.txt", "File to search.")
	flag.StringVar(&targetDir, "d", "D:/", "Search directory")
	flag.Parse()
}

func main() {
	// 读取参数

	fmt.Println("目标文件:", targetFile)
	fmt.Println("目标文件夹:", targetDir)
	fmt.Println("开始搜索。。。。。")
	flag.Parse()

	//查找文件
	// start := time.Now()
	// fuzzyseacherFile(targetFile, targetDir) //单线程的模糊搜索   实测运行6.77秒
	// fmt.Println("搜索结束，耗时：", time.Since(start))

	start1 := time.Now()
	workerCount = 1                                  //当前线程数设为1，因为我要开一个协程了
	go fastFuzzyseacher(targetFile, targetDir, true) //多线程的模糊搜索，这个协程为主协程    实测运行1.87秒
	//todo:这里我要等待所有的协程都结束，我要怎么做呢？
	waitForWorkers() //写一个等待函数来监听每一个channel的状态，如果所有的channel都关闭了，那么就说明所有的协程都结束了
	fmt.Println("多线程搜索结束，耗时：", time.Since(start1))

	// 打印文件内容

	if len(filelist) != 0 {
		for _, file := range filelist {
			fmt.Println(file)
		}

	} else {
		fmt.Println("没有找到文件")
	}

}

func fullnameSearchFile(targetFile string, targetDir string) []string {
	// 读取文件夹
	filelist := make([]string, 0)
	files, err := ioutil.ReadDir(targetDir)
	if err != nil {
		fmt.Println(err)

	}
	// 遍历文件夹
	for _, file := range files {
		// 判断是否为文件夹
		if file.IsDir() {
			// 递归
			fullnameSearchFile(targetFile, targetDir+file.Name()+"/")

		} else {
			// 判断是否为目标文件
			if file.Name() == targetFile {
				// 打印文件路径
				// fmt.Println(targetDir + file.Name())
				filelist = append(filelist, targetDir+file.Name())
				fmt.Println(filelist)
				fmt.Println("---------------------------")
			}

		}

	}
	return filelist
}

func fuzzyseacherFile(targetFile string, targetDir string) {

	// 读取文件夹
	files, err := ioutil.ReadDir(targetDir)
	if err != nil {
		fmt.Println(err)
	}
	// 遍历文件夹
	for _, file := range files {
		// 判断是否为文件夹
		if file.IsDir() {
			// 递归
			fuzzyseacherFile(targetFile, targetDir+file.Name()+"/")

		} else {
			// 判断是否为目标文件
			if strings.Contains(file.Name(), targetFile) {
				// 打印文件路径
				// fmt.Println(targetDir + file.Name())

				filelist = append(filelist, targetDir+file.Name())
				// fmt.Println("---------------------------")

				// foundMatch <- targetDir + file.Name() //找到了就把地址传给主线程，这里不要异步修改共享变量
			}

		}
	}

}

func waitForWorkers() { //主进程进行监控
	for {
		select { //使用select来监听channel

		case path := <-searchRequest: //需要新的线程了
			workerCount++
			go fastFuzzyseacher(targetFile, path, true)

		case <-workDone: //有线程结束了
			workerCount--
			// fmt.Println("线程结束，当前线程数：", workerCount)
			if workerCount == 0 { //假如所有的线程都结束了，退出等待，等主线程结束就行
				return
			}
		case filespath := <-foundMatch: //找到了
			filelist = append(filelist, filespath)
			// fmt.Println("当前列表", filelist)

		}
	}
}

func fastFuzzyseacher(targetFile string, targetDir string, master bool) { //多一个参数masster'，用来判断是否是主线程
	// 读取文件夹
	files, err := ioutil.ReadDir(targetDir)
	if err != nil {
		fmt.Println(err)
	}
	// 遍历文件夹
	for _, file := range files {
		// 判断是否为文件夹
		if file.IsDir() {
			// 递归
			//现在不可以无脑递归了，每次递归我要判断现在是否线程数超了
			if workerCount < maxWorkerCount { //如果当前线程数小于最大线程数，就开一个新的协程

				searchRequest <- targetDir + file.Name() + "/" //通过channel通信，把当前的目录传给下一个协程，相当于调协程池的入口

			} else { //这里注意！！！，如果不大于等于，不是什么都不做，而是继续当前的递归，只是不开协程而已，注意这里传false

				fastFuzzyseacher(targetFile, targetDir+file.Name()+"/", false)

			}

		} else {
			// 判断是否为目标文件
			if strings.Contains(file.Name(), targetFile) {
				// 打印文件路径
				// fmt.Println(targetDir + file.Name())
				// filelist = append(filelist, targetDir+file.Name())
				foundMatch <- targetDir + file.Name() //找到了就把地址传给主线程，这里不要异步修改共享变量
				// fmt.Println("---------------------------")
			}

		}

	}
	//这里是协程结束的地方,注意注意这里需要增加判断，由于是递归调用所以每个协程它的递归都会执行，所以我们开的协程调用的fastfuzzyseacher是true，而协程递归调用的方法则为false
	if master { //如果是主线程，就把主线程的channel关闭
		workDone <- true
	}

}
