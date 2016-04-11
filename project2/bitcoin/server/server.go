package main

import (
	"container/list"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/minhtrangvy/distributed_bitcoin_miner/project2/bitcoin"
	"github.com/minhtrangvy/distributed_bitcoin_miner/project2/lspnet"
	"github.com/minhtrangvy/distributed_bitcoin_miner/project2/lsp"
	"os"
)

type bitcoin_server struct {
	all_miners		map[int]*Message
	avail_miners	*list.List

	clients 		*list.List

	requests 		chan *Message
}

type bitcoin_client struct {
	client_id 		int
	assigned_miners	*list.List
}
//
// - miners + their job / whether they have a job
// - jobs + which clients they belong to
// - clients + their request
// - request queue
// - list of available miners w/o jobs
// TODO: how to determine if the client / miner have disconnected (esp. after the client has sent a request and is waiting for the result from the server)

func main() {
	const numArgs = 2
	if len(os.Args) != numArgs {
		fmt.Println("Usage: ./server <port>")
		return
	}
	var port string = os.Args[1]

	// Create a new server
	params := &lsp.Params{}
	server := lsp.NewServer(port, params)


}
