// // Contains the implementation of a LSP server.
//
// package lsp
//
// import "errors"
// import "encoding/json"
// import "fmt"
// // import "os"
// import "time"
// import "github.com/minhtrangvy/distributed_bitcoin_miner/project2/lspnet"
// import "strconv"
//
// type message2 struct {
// 	fromAddr 	*lspnet.UDPAddr
// 	message		Message
// }
//
// type server struct {
// 	connAddr		*lspnet.UDPAddr
// 	conn			*lspnet.UDPConn
// 	connections		map[int]*client
//
// 	nextserverID 	int
// 	// connectChan		(chan *Message)
//
// 	epochCh			(<-chan time.Time)
// 	numEpochs 		int
// 	lastEpoch		int
// 	epochLimit 		int
// 	epochMillis 	int
//
// 	toRead			(chan *message2)
// 	intermediateToRead (chan *message2) // TODO: take this out
// 	intermediateToSend (chan *message2)
//
// 	windowSize 		int
//
// 	closed			bool
// 	finished		bool
// }
//
// // NewServer creates, initiates, and returns a new server. This function should
// // NOT block. Instead, it should spawn one or more goroutines (to handle things
// // like accepting incoming server connections, triggering epoch events at
// // fixed intervals, synchronizing events using a for-select loop like you saw in
// // project 0, ets.) and immediately return. It should return a non-nil error if
// // there was an error resolving or listening on the specified port number.
// func NewServer(port int, params *Params) (Server, error) {
// 	// host, port, split_err = SplitHostPort(hostport)
// 	serverAddr, resolve_err := lspnet.ResolveUDPAddr("udp", "localhost:"+strconv.Itoa(port))
// 	PrintError(resolve_err, 66)
//
// 	// Server starts litening on given port
// 	connection, dial_err := lspnet.ListenUDP("udp", serverAddr)
// 	PrintError(dial_err, 59)
//
// 	current_server := server	{
// 		conn:			connection,
// 		connections:	make(map[int]*client),
//
// 		nextserverID: 	1,
//
// 		numEpochs: 		0,
// 		lastEpoch:		0,
// 		epochLimit: 	params.EpochLimit,
// 		epochMillis: 	params.EpochMillis,
//
// 		toRead:			make(chan *message2),
// 		intermediateToRead: make(chan *message2),
// 		intermediateToSend: make(chan *message2),
//
// 		windowSize: 	params.WindowSize,
//
// 		closed:			false,
// 		finished:		false,
// 	}
//
// 	go current_server.goServer()
// 	go current_server.goRead()
//
// 	curr_server := &current_server
// 	return curr_server, nil
// }
//
// func (s *server) Read() (int, []byte, error) {
// 	msg2 := <- s.toRead
// 	if msg2.message.SeqNum == 0 {
// 		return msg2.message.ConnID, nil, errors.New("not connected!")
// 	} else {
// 		return msg2.message.ConnID, msg2.message.Payload, nil
// 	}
// }
//
// func (s *server) Write(connID int, payload []byte) error {
// 	numToSend := s.connections[connID].seqNumToSend
// 	dataMessage := NewData(connID, numToSend, payload)
//
// 	if !s.closed {
// 		s.connections[connID].intermediateToSend <- dataMessage
// 	}
// 	return nil}
//
// func (s *server) CloseConn(connID int) error {
// 	s.connections[connID].closed = true
// 	return nil
// }
//
// func (s *server) Close() error {
// 	s.closed = true
// 	// TODO: add nil to all intermediateToRead of every client?
// 	s.toRead <- nil
// 	return errors.New("Closing server")
// }
//
// // ===================== GO FUNCTIONS ================================
//
// func (s *server) goServer() {
// 	s.epochCh = time.NewTicker(time.Duration(s.epochMillis)*time.Millisecond).C
// 	for !(s.closed && s.finished) {
// 		for connID, client := range s.connections {
// 			select {
// 			case <- s.epochCh:
// 				s.epoch()
// 			case msg2 := <- s.intermediateToRead: //TODO: actually loop through all the clients in this function
// 				if msg2 == nil {
// 					s.finished = true
// 					s.toRead <- nil
// 				} else {
// 					s.readHelper(*msg2)
// 				}
// 			case msg := <- client.intermediateToSend:
// 				sent := false
// 				for !sent {
// 					if len(client.dataWindow) < s.windowSize {
// 						s.writeHelper(*msg)
// 						sent = true;
// 					}
// 				}
// 			}
// 		}
// 	}
// 	fmt.Println("closing connection")
// 	s.conn.Close()
// }
//
// // always reading from the connection
// func (s *server) goRead(){
// 	for {
// 		if !(s.closed || s.finished) {
// 			// received_msg := s.readFromServer()
// 			buff := make([]byte, 1500)		// since packets should be <= 1500
// 			num_bytes, address, received_err := s.conn.ReadFromUDP(buff[0:])
// 			PrintError(received_err,270)
//
// 			s.lastEpoch = s.numEpochs
//
// 			received_msg := Message{}
// 			unmarshall_err := json.Unmarshal(buff[0:num_bytes], &received_msg)
// 			PrintError(unmarshall_err,273)
// 			received_msg2 := &message2{address,&received_msg}
// 			s.intermediateToRead <- &received_msg2
// 		} else {
// 			return
// 		}
// 	}
// }
//
// // ===================== HELPER FUNCTIONS ================================
//
// func (s *server) writeHelper(msg Message) {
// 	s.sendToServer(msg)
// 	s.dataWindow[msg.SeqNum] = msg
// }
//
// func (s *server) readHelper(msg2 message2) {
// 	msg := msg2.message
// 	fromAddr := msg2.fromAddr
// 	connID := msg.connID
// 	conn := s.connections[connID]
//
// 	switch msg.Type {
// 	// If we receive a data message, send an ack back
// 		// Make sure to add it to the ackWindow so we can resend acks during epoch
// 	case MsgData:
// 		// Make sure there is a connection
// 		if s.connID > 0 {
// 			maxAcceptableSN := s.expectedSN + s.windowSize
// 			// If not a duplicate, insert into toRead channel for Read to work with
// 			// Does it have to be within our window?
// 			if (msg.SeqNum >= s.expectedSN) && (msg.SeqNum <= maxAcceptableSN) {
// 				if msg.SeqNum == s.expectedSN {
// 					s.expectedSN++
// 				}
// 				s.toRead <- &msg
//
// 				// Send back an ack for the message
// 				ackMsg := NewAck(s.connID,msg.SeqNum)
// 				s.sendToServer(*ackMsg)
// 				s.moveAckWindow(*ackMsg)
// 			}
// 		} else {
// 			return
// 		}
// 	// If we receive an ack, indicate that the connection has been made if not already
// 		// If ack sn > 0, make sure that the data msg w/ the same sn leaves the unacked data window (if it was the oldest sn in the window, shift the window, update next expected SN)
// 	case MsgAck:
// 		// If this is an ack for the connection request
// 		if s.connID == 0 {
// 			s.connectChan <- &msg
// 			s.connID = msg.ConnID
// 			s.seqNumToSend = 1
// 			s.expectedSN = 1
// 		} else {
// 			s.deleteFromDataWindow(msg)
// 		}
// 	}
// }
//
// func (s *server) epoch() {
// 	s.numEpochs++
// 	// If no connection, resent connection request
// 	if s.connID == 0 {
// 		if (s.numEpochs - s.lastEpoch) > s.epochLimit {
// 			// lost connection, so close
// 			s.connectChan <- nil
// 			s.Close()
// 		} else {
// 			s.connectionRequest()
// 		}
// 	} else {
// 		if s.expectedSN == 1 {
// 			remindAck := NewAck(s.connID, 0)
// 			s.sendToServer(*remindAck)
// 		}
// 		for _, message := range s.dataWindow {
// 			s.sendToServer(message)
// 		}
// 		for _, ack := range s.ackWindow {
// 			s.sendToServer(ack)
// 		}
// 	}
// }
//
// func (s *server) connectionRequest() {
// 	connectMsg := NewConnect()
// 	connectMsg.ConnID = 0
// 	connectMsg.SeqNum = 0
// 	s.sendToServer(*connectMsg)
// }
//
// func (s *server) sendToServer(msg Message) {
// 	m_msg, marshal_err := json.Marshal(msg) // Marshalled connect message
// 	PrintError(marshal_err, 109)
// 	_, write_msg_err := s.conn.Write(m_msg)
// 	PrintError(write_msg_err, 111)
// }
//
// // func (s *server) readFromServer() (buffer []byte) {
// // 	buff := make([]byte, 1500)		// since packets should be <= 1500
// // 	num_bytes, _, received_err := s.conn.ReadFromUDP(buff[0:])
// // 	PrintError(received_err,270)
// //
// // 	s.lastEpoch = s.numEpochs
// //
// // 	received_msg := Message{}
// // 	unmarshall_err := json.Unmarshal(buff[0:num_bytes], &received_msg)
// // 	PrintError(unmarshall_err,273)
// // 	return &received_msg
// // }
//
// func (s *server) moveAckWindow(msg Message) {
// 	numAcksInWindow := len(s.ackWindow)
// 	// If the window is full, delete the oldest acknowledgement sent
// 	if numAcksInWindow == s.windowSize {
// 		lowestSN := s.findLowestSNinMap("ack")
// 		delete(s.ackWindow, lowestSN)
// 	}
// 	s.ackWindow[msg.SeqNum] = msg
// }
//
//
// func (s *server) deleteFromDataWindow(msg Message) {
// 	// Delete the data msg that was acked from the dataWindow
// 	delete(s.dataWindow, msg.SeqNum)
// }
//
// func (s *server) findLowestSNinMap(whichMap string) int {
// 	lowestSN :=  int(^uint(0) >> 1)
// 	currentMap := s.ackWindow
// 	if whichMap == "ack" {
// 		currentMap = s.ackWindow
// 	} else {
// 		currentMap = s.dataWindow
// 	}
// 	for sn, _ := range currentMap {
// 		if sn < lowestSN {
// 			lowestSN = sn
// 		}
// 	}
// 	return lowestSN
// }
//
// func PrintError(err error, line int) {
// 	if err != nil {
// 		fmt.Println("The error is: ", err, line)
// 	}
// }
//
// func ReturnError(err error, line int) error {
// 	if err != nil {
// 		PrintError(err,325)
// 		return err
// 	}
// 	return errors.New("Error")
// }

// Contains the implementation of a LSP server.

package lsp

import "errors"

type server struct {
	// TODO: implement this!
}

// NewServer creates, initiates, and returns a new server. This function should
// NOT block. Instead, it should spawn one or more goroutines (to handle things
// like accepting incoming client connections, triggering epoch events at
// fixed intervals, synchronizing events using a for-select loop like you saw in
// project 0, etc.) and immediately return. It should return a non-nil error if
// there was an error resolving or listening on the specified port number.
func NewServer(port int, params *Params) (Server, error) {
	return nil, errors.New("not yet implemented")
}

func (s *server) Read() (int, []byte, error) {
	// TODO: remove this line when you are ready to begin implementing this method.
	select {} // Blocks indefinitely.
	return -1, nil, errors.New("not yet implemented")
}

func (s *server) Write(connID int, payload []byte) error {
	return errors.New("not yet implemented")
}

func (s *server) CloseConn(connID int) error {
	return errors.New("not yet implemented")
}

func (s *server) Close() error {
	return errors.New("not yet implemented")
}
