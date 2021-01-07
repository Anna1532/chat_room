package main

import (
	"fmt"
	"net"
	"strings"
	"time"
)

//创建用户结构体类型
type Client struct {
	C    chan string
	Name string
	Addr string
}

//创建全局map，存储在线用户
var onlineMap map[string]Client

//创建全局channel，用于传递用户消息
var message = make(chan string)

//监听用户自带channel上是否有消息
func WriteMsg2Client(client Client, conn net.Conn) {
	for msg := range client.C {
		conn.Write([]byte(msg + "\n"))
	}
}

//提示信息
func MakeMsg(client Client, msg string) (buf string) {
	buf = "[" + client.Addr + "]" + client.Name + ": " + msg
	return buf
}

func HandlerConnect(conn net.Conn) {
	defer conn.Close()
	//创建channel判断用户是否活跃
	hasData := make(chan bool)
	//获取用户网络地址
	netAddr := conn.RemoteAddr().String()
	//创建新连接用户的结构体,默认使用用户的IP+端口号作为用户名
	client := Client{make(chan string), netAddr, netAddr}
	//将新连接用户添加到在线用户map中，key:IP+port, value:client
	onlineMap[netAddr] = client

	//创建用来给当前用户发送消息的goroutine
	go WriteMsg2Client(client, conn)

	//发送用户上线消息到全局channel中
	message <- MakeMsg(client, "login")

	//创建一个channel，判断用户退出状态
	isQuit := make(chan bool)

	//创建一个匿名goroutine，专门处理用户发送的消息
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := conn.Read(buf)
			if n == 0 {
				isQuit <- true
				fmt.Printf("检测到客户端:%s 退出\n", client.Name)
				return
			}
			if err != nil {
				fmt.Println("conn Read error:", err)
				return
			}
			//将读到的用户消息，保存到msg中，stirng类型
			msg := string(buf[:n-1]) //删掉"\n"字符
			//提取在线用户列表
			if msg == "who" && len(msg) == 3 {
				conn.Write([]byte("user list:\n"))
				//遍历当前map，获取当前用户
				for _, user := range onlineMap {
					userInfo := user.Addr + ":" + user.Name + "\n"
					conn.Write([]byte(userInfo))
				}
			} else if len(msg) >= 8 && msg[:6] == "rename" {
				//判断用户发送了改名命令
				newname := strings.Split(msg, "|")[1]
				client.Name = newname
				onlineMap[netAddr] = client
				conn.Write([]byte("your name has changed\n"))
			} else {
				//将读到的用户消息广播给所有在线用户，写入到message
				message <- MakeMsg(client, msg)
			}
			hasData <- true
		}
	}()

	for {
		//监听channel上的数据流动
		select {
		case <-isQuit:
			delete(onlineMap, client.Addr)       //将用户从在线用户列表移除
			message <- MakeMsg(client, "logout") //写入用户退出消息到全局channel
			return
		case <-hasData:
			//什么都不做，目的是重置下面case的计时器
		case <-time.After(time.Second * 10):
			delete(onlineMap, client.Addr)       //将用户从在线用户列表移除
			message <- MakeMsg(client, "logout") //写入用户退出消息到全局channel
			return
		}
	}
}

func Manager() {
	//初始化onlineMap
	onlineMap = make(map[string]Client)
	//监听全局channel中是否有数据,有数据存储到msg，无数据阻塞
	for {
		msg := <-message
		//循环发送消息给所有在线用户
		for _, client := range onlineMap {
			client.C <- msg
		}
	}
}

func main() {
	//创建监听套接字
	listener, err := net.Listen("tcp", "127.0.0.1:8000")
	if err != nil {
		fmt.Println("Listen err", err)
		return
	}
	defer listener.Close()

	//创建管理者goroutine，管理map和全局channel
	go Manager()

	//循环监听客户端连接请求
	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Accept err", err)
			return
		}
		//启动goroutine处理客户端数据请求
		go HandlerConnect(conn)
	}
}
