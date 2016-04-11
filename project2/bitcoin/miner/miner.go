package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/minhtrangvy/distributed_bitcoin_miner/project2/bitcoin"
	"github.com/minhtrangvy/distributed_bitcoin_miner/project2/lspnet"
	"github.com/minhtrangvy/distributed_bitcoin_miner/project2/lsp"
	"os"
)

const (
    name = "log.txt"
    flag = os.O_RDWR | os.O_CREATE
    perm = os.FileMode(0666)
	maxUint64 = 1<<64 - 1
)

func main() {
	const numArgs = 2
	if len(os.Args) != numArgs {
		fmt.Println("Usage: ./miner <hostport>")
		return
	}
	string hostport = os.Args[1]

	// Create miner
	params := &lsp.Params{}
	miner, miner_err = lsp.NewClient(hostport, params)
	printError(miner_err, "Failed to create miner.")

	// Create a join request and send to server
	request := bitcoin.NewJoin()
	m_msg, marshal_err := json.Marshal(request)
	printError(marshal_err, "Failed to marshal message in miner.")
	_, write_msg_err := miner.Write(m_msg)
	printError(write_msg_err, "Failed to write message from miner to server.")

	workWorkWorkWorkWork()
}

func workWorkWorkWorkWork() {
	for {
		var min_nonce uint64 = maxUint64
		var min_hash uint64 = maxUint64

		// Read job request from server
		job, read_err := miner.Read()
		if read_err != nil {
			printError(read_err, "Failed to read job from server.")
			miner.Close()
		}
		job_msg := bitcoin.NewRequest{}
		unmarshal_err := json.Unmarshal(job, &job_msg)
		printError(unmarshal_err,"Failed to unmarshal job message")

		// Compute min hash for given nonce range
		for (i := job_msg.Lower; i < job_msg.Upper; i++) {
			current_hash = bitcoin.Hash(job_msg.Data, i)
			if current_hash < min_hash {
				min_nonce = i
				min_hash = current_hash
			}
		}

		// Send result back to server
		result := bitcoin.NewResult(min_hash, min_nonce)
		m_msg, marshal_err := json.Marshal(result)
		printError(marshal_err, "Failed to marshal result in miner.")
		_, write_msg_err := miner.Write(m_msg)
		printError(write_msg_err, "Failed to write result from miner.")
	}
}

// hash := bitcoin.Hash("thom yorke", 19970521)
// msg := bitcoin.NewRequest("jonny greenwood", 200, 71010)

// ==================================== HELPER FUNCTIONS ======================================================
func printError(err error, debugInfo string) {
	if err != nil {
		fmt.Printf(debugInfo)
	}
}

func logDebugInfo(info string) {
	file, err := os.OpenFile(name, flag, perm)
	if err != nil {
	    return
	}

	LOGF := log.New(file, "", log.Lshortfile|log.Lmicroseconds)
	LOGF.Println(info)

	file.Close()
}
