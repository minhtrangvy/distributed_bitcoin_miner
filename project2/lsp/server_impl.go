package lsp

import "errors"
import "encoding/json"
import "fmt"
import "os"
import "time"
import "github.com/minhtrangvy/distributed_bitcoin_miner/project2/lspnet"
import "strconv"

// type client struct {
// 	connID 			int
// 	connAddr		*lspnet.UDPAddr
// 	lowestUnackSN	int
// 	currWriteSN		int 					// The SN of the message to write to the client
// 	expectedSN	 	int
// 	writeCh			(chan *Message)
// 	closeCh			(chan int)				// Close() has been called
// 	dataWindow		map[int]*Message
// 	ackWindow		map[int]*Message
// }

type server struct {
	connection 		*lspnet.UDPConn
	numClients		int
	clients			map[int]*client
	clientsAddr		map[*lspnet.UDPAddr]int
	lowestUnackSN 	int
	allAck			bool

	readCh	 		(chan *Message)

	closeCh			(chan int)				// Close() has been called
	isClosed		bool
	intermedReadCh 	(chan *Messages)

	epochCh			(<-chan time.Time)
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
	go current_server.read()
	go current_server.epoch()

	return current_server, nil
}

func (s *server) Read() (int, []byte, error) {
	select {
	// Connection to some client has been explicitly closed
	case <- s.closeCh:
		return 0, nil, errors.New("Channel has been closed")
	// Connection has been lost due to an epoch timeout and no other


	// // cases for closing
	// 	// TODO return -1, nil, errors.New("client closed or something")
	case msg <- s.intermedReadCh:
		return msg.ConnID, msg.Payload, nil
	}
}

func (s *server) Write(connID int, payload []byte) error {
	msg := NewData(connID, s.clients[connID].currWriteSN, payload)
	s.clients[connID].writeCh <- msg
	s.clients[connID].currWriteSN++
	return nil 				// TODO: Return non-nil error when connection with connID is lost
}

func (s *server) CloseConn(connID int) error {
	s.clients[connID].closeCh <- 1
	return nil
}

func (s *server) Close() error {
	s.isClosed = true
	return nil
}

// ===================== GO ROUTINES ================================

func (s *server) read() {
	for {
		select {
			case <- s.closeCh:
				// Loop through all clients and tell them to close
				for _, client := range s.clients {
					client.closeCh <- 1
				}

				s.closeCh <- 1 					// very hacky: we are putting it back in order to close the other go routines
				return

			default:
				buff := make([]byte, 1500)
				num_bytes_received, client_addr, received_err := s.connection.ReadFromUDP(buff[0:])

				// If we fail to read from the connection, we want to disconnect from the client, because
				// it has been closed.
				if received_err != nil {
					s.clients[s.clientsAddr[client_addr]].closeCh <- 1
					fmt.Fprintf(os.Stderr, "Server failed to read from the client. Exit code 2.", received_err)
				}

				received_msg := Message{}
				unmarshal_err := json.Unmarshal(buff[0:num_bytes_received], &received_msg)
				s.PrintError(unmarshal_err)

				// If the message type is Connect, deal with it here
				if received_msg.Type == MsgConnect {
					if _, ok := s.clientsAddr[client_addr]; ok {
						if msg.SeqNum == 0 {
							s.numClients++

							curr_client := &client{
								connID:		 s.numClients,
								address: 	 client_addr,

								currWriteSN: 1,
								expectedSN:	 1,

								closeCh:	 make(chan int),
								isClosed:	 false,

								numEpochs: 	0,
							}
						}

						ackMsg := NewAck(curr_client.connID, 0)
						s.sendMessage(ackMsg)

						s.clients[curr_client.connID] = curr_client
						s.clientsAddr[client_addr] = curr_client.connID

						go clientHandler(curr_client.connID)
					}

				// Otherwise, put it into the read channel
				} else {
					currentID := s.clientsAddr[client_addr]
					s.clients[currentID].readCh <- &received_msg
				}
		}
	}
}

func (s *server) clientHandler(clientID int) {
	s.clients[clientID].epochCh = time.NewTicker(time.Duration(c.epochMilli) * time.Millisecond).C
	for {
	case msg := <- s.clients[clientID].readCh:
		currentSN := msg.SeqNum
		// TODO: is clientID and msg.ConnID the same?
		switch msg.Type {
		case MsgAck:
			if _, ok := s.clients[clientID].dataWindow[currentSN]; ok {
				delete(s.clients[clientID].dataWindow, currentSN)

				// If we received an ack for the oldest unacked data msg
				if currentSN == s.clients[clientID].lowestUnackSN {
					if len(s.clients[clientID].dataWindow) == 0 {
						s.clients[clientID].lowestUnackSN = s.findNewMin(s.clients[clientID].dataWindow)
					} else {
						s.clients[clientID].lowestUnackSN++
					}
				}

				// Check if all data message have been sent and acknowledged. If so, the
				// server can be closed
				readyToClose := true
				for _, client := range s.clients {
					if !(s.isClosed && len(client.writeCh) == 0 && len(client.dataWindow) == 0) {
						readyToClose = false
						break
					}
				}

				if readyToClose {
					s.closeCh <- 1
				}
			}
		case MsgData:
			// Drop any message that isn't the expectedSN
			s.clients[currentConnID].numEpochs = 0
			if (currentSN == s.clients[clientID].expectedSN) {
				s.intermedReadCh <- msg
				s.clients[currentConnID].expectedSN++

				ackMsg := NewAck(currentConnID, currentSN)
				s.sendMessage(ackMsg)

				oldestAckSN := s.findNewMin(s.clients[currentConnID].ackWindow)
				delete(s.clients[currentConnID].ackWindow, oldestAckSN)
				s.clients[currentConnID].ackWindow[currentSN] = ackMsg
		}
	case msg := <- s.clients[clientID].writeCh:
		m_msg, marshal_err := json.Marshal(msg)
		s.PrintError(marshal_err)
		n, write_err = s.connection.WriteToUDP(m_msg, s.clients[connID].address)

		// If we fail to write to a client, that means the client has been closed or
		// connection has been lost.
		if write_msg_err != nil {
			s.clients[clientID].closeCh <- 1

			fmt.Printf("Current client ID is %d\n", clientID)
			fmt.Fprintf(os.Stderr, "Server failed to write to the client. Exit code 1.", write_msg_err)
		}

	// If an epoch happens for that client
	case <- s.clients[clientID].epochCh:
		// If the numEpochs has reached the limit, we need to disconnect
		// from the connection
		if s.clients[clientID].numEpochs >= s.epochLimit {
			s.clients[clientID].closeCh <- 1

		} else {
			// If no data messages have been received from the client, then resend an ack msg for
			// 	the client's connection request
			if s.clients[clientID].expectedSN == 1 {
				ackMsg := NewAck(clientID, 0)
				s.sendMessage(ackMsg)
			}

			// For each data message that has been sent but not yet acknowledged,
			// resend the data message
			for _, value := range s.clients[clientID].dataWindow {
				s.sendMessage(value)
			}

			// Resend an acknowledgement message for each of the last w (or fewer)
			// distinct data messages that have been received
			for _, value := range s.clients[clientID].ackWindow {
				s.sendMessage(value)
			}

			s.clients[clientID].numEpochs++
		}

	// TODO: This could be bad
	// When the client receives something in its close channel, close the client.
	case <- s.clients[clientID].closeCh:
		s.clients[clientID].Close()

		delete(s.clientsAddr, s.clients[clientID].address)
		delete(s.clients, clientID)
	}
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
