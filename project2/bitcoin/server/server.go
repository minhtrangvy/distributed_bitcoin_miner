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

const (
    flag = os.O_RDWR | os.O_CREATE
    perm = os.FileMode(0666)
	maxUint64 = 1<<64 - 1
	minerLoad = uint64(24)
)

type bitcoin_server struct {
	all_miners		map[int]*bitcoin_job 		/* A map of miner ID to the job */
	avail_miners	*list.List 					/* A list of miner IDs */
	clients 		map[int]*bitcoin_client
	requests 		chan *bitcoin_job
}

type bitcoin_job struct {
	client_id		int
	message 		*Message
}

type bitcoin_client struct {
	client_id 		int
	minHash 		uint64
	minNonce		uint64
	assigned_miners	*list.List 					/* A list of miner IDs */

	// data			string
	// lower			uint64
	// upper			uint64
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

	bit_server := &bitcoin_server{all_miners: make(map[int]*bitcoin_job),
								  avail_miners: new list.List(),
								  clients: new list.List(),
								  requests: make(chan *Message),}

	go server.acceptMessages(bit_server)
	go server.scheduleJobs(bit_server)
}

func (s *lsp.Server) acceptMessage(bit_server &bitcoin_server) {
	for {
		conn_id, read_msg, read_err := s.Read()
		if read_err != nil {
			printError(read_err, "Failed to read message in server.")
			s.Close()
		}

		// Print out results
		var message *Message
		unmarshal_err := json.Unmarshal(read_msg, &message)
		printError(unmarshal_err,"Failed to unmarshal message")
		}

		switch message.Type {
		case Join:							/* Miner asks to join */
			bit_server.all_miners[conn_id] = nil
			bit_server.avail_miners.PushBack(conn_id)

		case Request:						/* Client requests a job */
			bit_client := &bitcoin_client{client_id: conn_id,
										  minHash: maxUint64,
										  minNonce: maxUint64,
										  assigned_miners: new list.List(),}
										   // data: message.Data,
										   // lower: message.Lower,
										   // upper: message.Upper}

			clients[conn_id] = bit_client
			requests.PushBack(&bitcoin_job{client_id: conn_id,
										   message: message})

		case Result:						/* Receive result from miner */
			givenHash := message.Hash
			givenNonce := message.Nonce

			// Identify the client associated with the miner's job
			job := all_miners[conn_id]
			client_id := job.client_id

			// Determine whether the job returns a new minimum 
			// nonce and hash for the client
			if givenHash < clients[client_id].minHash {
				clients[client_id].minHash = givenHash
				clients[client_id].minNonce = givenNonce
			}

			// Delete the miner from the client's list of miners and
			// add the miner to the list of available miners
			clients[client_id].assigned_miners.Remove(conn_id)
			s.avail_miners.PushBack(conn_id)

			// If all the client's miners are done and the connection
			// still exists, then write result back to client
			if clients[client_id].assigned_miners.Length() == 0 {
				result := bitcoin.NewResult(clients[client_id].minHash, clients[client_id].minNonce)
				m_msg, marshal_err := json.Marshal(result)
				printError(marshal_err, "Failed to marshal message.")

				_, write_msg_err := s.Write(m_msg)
				printError(write_msg_err, "Failed to write message to server.")
				
				// If write fails, the client is disconnected
				if write_msg_err != null {
					delete(clients, client_id)
				}
			}
		}

}

// When we send a request to the miner and it fails, we know the miner has disconnected
func (s *lsp.Server) scheduleJobs(bit_server &bitcoin_server) {
	for {
	case curr_job := <-requests:		/* When a request is received */
		// Extract information from request
		client_id := curr_job.client_id
		minNonce := curr_job.message.Lower
		maxNonce := curr_job.message.Upper

		// Assign request to an available miner
		numJobs := (maxNonce - minNonce)/minerLoad
		for i := 0; i < numJobs; i++ {
			assigned := false
			for !(assigned) {
				if avail_miners.Length() > 0 {
					miner_id := avail_miners.Front()
					assigned = true

					// Send request to server
					request := bitcoin.NewRequest(curr_job.message.Data, minNonce, maxNonce)
					m_msg, marshal_err := json.Marshal(request)
					printError(marshal_err, "Failed to marshal message.")

					_, write_msg_err := client.Write(m_msg)
					printError(write_msg_err, "Failed to send request to miner.")
					// If we fail to send the request to server, the miner is closed
					if write_msg_err != nil {
						delete(avail_miners, miner_id)
						all_miners.Remove(miner_id)
					}

					// Update assignments in structs
					all_miners[miner_id] = &bitcoin_job{client_id: client_id,
														message: curr_job.message}
					s.clients.assigned_miners.PushBack(miner_id)
				}
			}
		}

	default:
	}
}

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











