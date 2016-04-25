package lsp

import "errors"
import "encoding/json"
import "fmt"
import "os"
import "time"
import "github.com/minhtrangvy/distributed_bitcoin_miner/project2/lspnet"
import "strconv"

type client struct {
	connID 			int
	connAddr		*lspnet.UDPAddr
	lowestUnackSN	int

	currWriteSN		int 					// The SN of the message to write to the client
	expectedSN	 	int
	writeCh			(chan *Message)

	closeCh			(chan int)				// Close() has been called

	dataWindow		map[int]*Message
	ackWindow		map[int]*Message
}

type server struct {
	connection 		*lspnet.UDPConn
	numClients		int
	clients			map[int]*client
	lowestUnackSN 	int
	expectedSN	 	int

	allAck			bool

	readCh	 		(chan *Message)

	closeCh			(chan int)				// Close() has been called
	isClosed		bool

	epochCh			(<-chan time.Time)

	numEpochs		int 					// number of epochs that have occurred
	windowSize		int
	epochMilli		int
	epochLimit		int
}

// NewServer creates, initiates, and returns a new server. This function should
// NOT block. Instead, it should spawn one or more goroutines (to handle things
// like accepting incoming client connections, triggering epoch events at
// fixed intervals, synchronizing events using a for-select loop like you saw in
// project 0, etc.) and immediately return. It should return a non-nil error if
// there was an error resolving or listening on the specified port number.
func NewServer(port int, params *Params) (Server, error) {
	current_server := &server {
		numClients:		0,
		clients:	    make(map[int]*client),
		lowestUnackSN: 	0,
		expectedSN:	 	0,

		allAck:			false,

		readCh:			make(chan *Message), 		// data messages to be written to server
		writeCh:		make(chan *Message),

		closeCh:		make(chan int),				// Close() has been called
		isClosed:		false,

		epochCh:		make(<-chan time.Time),
		dataWindow:		make(map[int]*Message),		// map from SN to *Message of unacknowledged data messages we have sent
		ackWindow:		make(map[int]*Message),		// map of the last windowSize acks that we have sent

		numEpochs:		0, 					// number of epochs that have occurred
		windowSize: 	params.WindowSize,
		epochMilli: 	params.EpochMillis,
		epochLimit: 	params.EpochLimit,
	}

	serverAddr, resolve_err := lspnet.ResolveUDPAddr("udp", port)
	current_server.PrintError(resolve_err)

	// Send connect message to server
	current_conn, listen_err := lspnet.ListenUDP("udp", serverAddr)
	current_server.PrintError(listen_err)

	current_server.connection = current_conn

	// Go routines
	go current_server.master()
	go current_server.read()
	go current_server.epoch()

	return current_server, nil
}

func (s *server) Read() (int, []byte, error) {
	// TODO: remove this line when you are ready to begin implementing this method.
	select {} // Blocks indefinitely.

	// Pull from write channel 
	return -1, nil, errors.New("not yet implemented")
}

func (s *server) Write(connID int, payload []byte) error {
	msg := NewData(connID, s.clients[connID].currWriteSN, payload)
	s.writeCh <- msg

	s.clients[connID].currWriteSN++
	return nil 				// TODO: Return non-nil error when connection with connID is lost
}

func (s *server) CloseConn(connID int) error {
	return errors.New("not yet implemented")
}

func (s *server) Close() error {
	return errors.New("not yet implemented")
}

// ===================== GO ROUTINES ================================

func (s *server) master() {
	for {
		case msg := <- s.readCh:
			currentSN := msg.SeqNum
			currentConnID := msg.ConnID

			switch msg.Type {
			case MsgAck:
				if _, ok := s.clients[currentConnID].dataWindow[currentSN]; ok {
					delete(s.clients[currentConnID].dataWindow, currentSN)

					// If we received an ack for the oldest unacked data msg
					if currentSN == s.clients[currentConnID].lowestUnackSN {
						if len(s.clients[currentConnID].dataWindow) == 0 {
							s.clients[currentConnID].lowestUnackSN = s.findNewMin(s.clients[currentConnID].dataWindow)
						} else {
							s.clients[currentConnID].lowestUnackSN++
						}
					}
				}

			case MsgData:
				// Drop any message that isn't the expectedSN
				if (currentSN == s.clients[currentConnID].expectedSN) {
					c.intermedReadCh <- msg
					c.expectedSN++

					ackMsg := NewAck(c.connID, currentSN)
					c.sendMessage(ackMsg)

					oldestAckSN := c.findNewMin(c.ackWindow)
					delete(c.ackWindow, oldestAckSN)
					c.ackWindow[currentSN] = ackMsg
				}

			default:

		}
	}
}

func (s *server) read() {
	for {
		select {
			case <- s.closeCh:
				c.closeCh <- 1 					// very hacky: we are putting it back in order to close the other go routines
				return
			default:
				buff := make([]byte, 1500)
				num_bytes_received, client_addr, received_err := s.connection.ReadFromUDP(buff[0:])
				if received_err != nil {
					fmt.Fprintf(os.Stderr, "Server failed to read from the client. Exit code 2.", received_err)
					os.Exit(2)
				}

				received_msg := Message{}
				unmarshal_err := json.Unmarshal(buff[0:num_bytes_received], &received_msg)
				s.PrintError(unmarshal_err)

				// If the message type is Connect, deal with it here
				if received_msg.Type == MsgConnect {
					if msg.SeqNum == 0 {
						numClients++

						curr_client := &client{
							connID:		 s.numClients,
							connAddr: 	 client_addr,

							currWriteSN: 1,
							expectedSN:	 0,

							closeCh:	 make(chan int),
							isClosed:	 false,
						}

						ackMsg := NewAck(curr_client.connID, 0)
						s.sendMessage(ackMsg)

						s.clients[curr_client.connID] = curr_client
					}

				// Otherwise, put it into the read channel
				} else {
					s.readCh <- &received_msg
				}
		}
	}
}

func (s *server) epoch() {

}

// ===================== HELPER FUNCTIONS ================================
func (s *server) findNewMin(currentMap map[int]*Message) int {
	currentLowest := MaxInt
	for key, _ := range currentMap {
		if key < currentLowest {
			currentLowest = key
		}
	}
	return currentLowest
}

func (s *server) sendMessage(msg *Message) {
	m_msg, marshal_err := json.Marshal(msg)
	s.PrintError(marshal_err)
	_, write_msg_err := s.connection.Write(m_msg)
	if write_msg_err != nil {
		fmt.Fprintf(os.Stderr, "Server failed to write to the client. Exit code 1.", write_msg_err)
		os.Exit(1)
	}
}

func (s *server) PrintError(err error) {
	if err != nil {
		fmt.Println("The error is: ", err)
	}
}

func (s *server) ReturnError(err error, line int) error {
	if err != nil {
		s.PrintError(err)
		return err
	}
	return errors.New("Error")
}

