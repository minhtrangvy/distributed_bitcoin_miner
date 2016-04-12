package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/minhtrangvy/distributed_bitcoin_miner/project2/bitcoin"
	"github.com/minhtrangvy/distributed_bitcoin_miner/project2/lspnet"
	"github.com/minhtrangvy/distributed_bitcoin_miner/project2/lsp"
	"os"
	"strconv"
)

func main() {
	const numArgs = 4
	if len(os.Args) != numArgs {
		fmt.Println("Usage: ./client <hostport> <message> <maxNonce>")
		return
	}

	// Grab arguments
	var hostport string = os.Args[1]
	var message string = os.Args[2]
	var maxNonce string = os.Args[3]
	var maxNonce uint64 = strconv.ParseInt(maxNonce, 10, 64)

	// Create client
	params := &lsp.Params{}
	client, client_err = lsp.NewClient(hostport, params)
	printError(client_err, "Failed to create client.")

	// Send request to server
	request := bitcoin.NewRequest(message, uint64(0), maxNonce)
	m_msg, marshal_err := json.Marshal(request)
	printError(marshal_err, "Failed to marshal message.")
	_, write_msg_err := client.Write(m_msg)
	printError(write_msg_err, "Failed to write message to server.")

	// Wait for results from server
	results, read_err := client.Read()
	if read_err != nil {
		printError(read_err, "Failed to read results from server.")
		client.Close()
	}
	// Print out results
	result_msg := bitcoin.NewResult{}
	unmarshal_err := json.Unmarshal(results, &result_msg)
	printError(unmarshal_err,"Failed to unmarshal message")

	printResult(result_msg.Hash, result_msg.Nonce)

	client.Close() 		// TODO: is it os.Exit()???
}

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

// printResult prints the final result to stdout.
func printResult(hash, nonce string) {
	fmt.Println("Result", hash, nonce)
}

// printDisconnected prints a disconnected message to stdout.
func printDisconnected() {
	fmt.Println("Disconnected")
}
