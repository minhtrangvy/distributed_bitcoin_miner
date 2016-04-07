// Contains the implementation of a LSP client.

package lsp

import "errors"
import "encoding/json"
import "fmt"
import "os"
import "time"
import "reflect"

type client struct {
	connID			int
	conn			*lspnet.UDPConn
	connAddr		*lspnet.UDPAddr

	seqNumToSend	int
	expectedSN		int		// the next SN we expect to receive

	connectChan		(chan Message)

	epochCh			(chan int)
	numEpochs 		int
	lastEpoch		int
	epochLimit 		int
	epochMillis 	int

	windowSize 		int
	ackWindow		map[int]Message
	dataWindow 		map[int]Message

	intermediateToRead (chan Message)
	intermediateToSend (chan Message)
	toRead			(chan Message)
	toWrite			(chan Message)

	lostConn		bool
	closed 			bool
}

// NewClient creates, initiates, and returns a new client. This function
// should return after a connection with the server has been established
// (i.e., the client has received an Ack message from the server in response
// to its connection request), and should return a non-nil error if a
// connection could not be made (i.e., if after K epochs, the client still
// hasn't received an Ack message from the server in response to its K
// connection requests).
//
// hostport is a colon-separated string identifying the server's host address
// and port number (i.e., "localhost:9999").
func NewClient(hostport string, params *Params) (Client, error) {
	// host, port, split_err = SplitHostPort(hostport)
	serverAddr, resolve_err := net.ResolveUDPAddr("udp", hostport)
	PrintError(resolve_err, 28)

	// Send connect message to server
	conn, dial_err = DialUDP("udp", nil, hostport)

	current_client := client	{
		// connID:         0,
		// conn:			conn,
		// connectChan:	make(chan *Message,1),
		//
		// seqNumToSend:	0,			// next Sn to send
		// expectedSN:		0,
		//
		// epochCh: 		make(chan int, 1),
		// numEpochs: 		0,
		// epochLimit: 	params.EpochLimit,
		// epochMillis:	params.EpochMillis,
		//
		// windowSize: 	params.WindowSize,
		// ackWindow:		make(map[int]Message),
		// dataWindow:		make(map[int]Message),
		//
		// intermediateToRead: make(chan *Message, 1),
		// toRead:			make(chan *Message, 1),
		// toWrite:		make(chan *Message, 1),
		//
		// lostConn:		make(chan int, 1),
		// closed:			make(chan int, 1),
	}

	// for index,_ := range current_client.dataWindow {
	// 	current_client[index].occupiedSlot = 0
	// }

	go goClient(&current_client)
	go goRead(&current_client)

	// Send connection request to server
	connectionRequest(conn)

	// Block until we get a connection ack back
	connectAck := <- connectChan

	// Then return the appropriate client
	if connectAck == nil {
		return nil, errors.New("Could not connect!")
	} else {
		current_client.connID = connectAck.ConnID
		return current_client, nil
	}
}

func (c *client) ConnID() int {
	return c.connID
}

func (c *client) Read() ([]byte, error) {
	msg := <- c.toRead
	if msg == nil {
		return nil, errors.New("(1) the connection has been explicitly closed, or (2) the connection has been lost due to an epoch timeout and no other messages are waiting to be returned.")
	} else {
		return msg.Payload, nil
	}
}

func (c *client) Write(payload []byte) error {
	dataMessage := NewData(c.connID, c.seqNumToSend, payload)
	// if len(c.dataWindow) < c.windowSize {
	// 	c.toWrite <- dataMessage
	// } else {

	if !c.closed {
		c.intermediateToSend <- dataMessage
	}
	// Create a data messages
	// Cannot send unless dataWindow < windowSize
		// If it is, put msg into intermediateToSend
				//helper:// Else Send it
						// Add it it to our window
	return errors.New("not yet implemented")
}

func (c *client) Close() error {
	c.closed = true
	c.intermediateToRead <- nil
	return errors.New("Closing client")
}

// ===================== GO FUNCTIONS ================================

func (c *client) goClient() {
	epochCh = time.NewTicker(c.epochMillis).C
	for !(c.closed){
		select {
		case <- c.epochCh:
			c.epoch()
		case msg <- c.intermediateToRead:
			if msg == nil {
				c.lostConn = true
				c.toRead <- nil
			} else {
				c.readHelper(msg)
			}
		case ((len(c.dataWindow) < c.windowSize) && (msg <- c.intermediateToSend)):
			c.writeHelper(msg)
		}
	}

	c.conn.close()
}

// always reading from the connection
func (c *client) goRead(){
	for {
		select {
		case <- c.lostConn:
			return
		case <- c.closed:
			return
		default:
			received_msg := c.readFromServer()
			c.intermediateToRead <- msg
		}
	}
}

// ===================== HELPER FUNCTIONS ================================

func (c *client) writeHelper(msg Message) {
	c.sendToServer(msg)
	c.dataWindow[msg.SeqNum] = msg
}

func (c *client) readHelper(msg Message) {
	select {
	// If we receive a data message, send an ack back
		// Make sure to add it to the ackWindow so we can resend acks during epoch
	case msg.Type == MsgData:
		// Make sure there is a connection
		if c.connID > 0 {
			maxAcceptableSN := c.expectedSN + c.windowSize
			// If not a duplicate, insert into toRead channel for Read to work with
			// Does it have to be within our window?
			if (msg.SeqNum >= c.expectedSN) && (msg.SeqNum <= maxAcceptableSN) {
				if msg.SeqNum == c.expectedSN {
					c.expectedSN++
				}
				c.toRead <- msg

				// Send back an ack for the message
				ackMsg := NewAck()
				ackMsg.ConnID = c.connID
				ackMsg.SeqNum = msg.SeqNum
				c.sendToServer(ackMsg)
				c.moveAckWindow(ackMsg)
			}
		} else {
			return
		}
	// If we receive an ack, indicate that the connection has been made if not already
		// If ack sn > 0, make sure that the data msg w/ the same sn leaves the unacked data window (if it was the oldest sn in the window, shift the window, update next expected SN)
	case msg.Type == MsgAck:
		// If this is an ack for the connection request
		if c.connID == 0 {
			c.connID = msg.ConnID
			c.seqNumToSend = 1
			c.expectedSN = 1
		} else {
			c.deleteFromDataWindow(msg)
		}
	}
}

func (c *client) epoch() {
	c.numEpochs++
	// If no connection, resent connection request
	if c.connID == 0 {
		if (c.numEpochs - c.lastEpoch) > c.epochLimit {
			c.lostConn <- 1 		// connection lost
			c.Close()
		} else {
			c.connectionRequest()
		}
	} else {
		if c.expectedSN == 1 {
			remindAck := NewAck(c.connID, 0)
		}
		for sn, message := range c.dataWindow {
			c.sendToServer(message)
		}
		for sn, ack := range c.ackWindow {
			c.sendToServer(ack)
		}
	}
}

func (c *client) connectionRequest() {
	connectMsg := NewConnect()
	connectMsg.ConnID = 0
	connectMsg.SeqNum = 0
	bytes_written = c.sendToServer(connectMsg)
}

func (c *client) sendToServer(msg Message) {
	m_msg, marshal_err := json.Marshal(msg) // Marshalled connect message
	PrintError(marshal_err, 109)
	bytes_written, write_msg_err := conn.Write(m_msg)
	PrintError(write_msg_err, 111)
	return bytes_written
}

func (c *client) readFromServer() (buffer []byte) {
	buff := make([]byte, 1500)		// since packets should be <= 1500
	num_bytes, received_addr, received_err := c.conn.lspnet.ReadFromUDP(buff[0:])
	c.lastEpoch = c.numEpochs
	PrintError(received_err)
	received_msg := Message{}
	json.Unmarshal(buff[0:num_bytes], &received_msg)
	return received_msg
}

func (c *client) moveAckWindow(msg Message) {
	numAcksInWindow := len(c.ackWindow)
	// If the window is full, delete the oldest acknowledgement sent
	if numAcksInWindow == c.windowSize {
		lowestSN := findLowestSNinMap("ack")
		delete(c.ackWindow[lowestSN])
	}
	c.ackWindow[msg.SeqNum] = msg
}


func (c *client) deleteFromDataWindow(msg Message) {
	// Delete the data msg that was acked from the dataWindow
	delete(c.dataWindow[msg.SeqNum]
}

func (c *client) findLowestSNinMap(whichMap string) int {
	lowestSN := Inf(0)
	if whichMap == "ack" {
		currentMap := c.ackWindow
	} else {
		currentMap := c.dataWindow
	}
	for sn, curr_msg := range currentMap {
		lowestSN = min(lowestSN,sn)
	}
	return lowestSN
}

func PrintError(err error, line int) {
	if err != nil {
		fmt.Println("The error is: ", err, line)
	}
}

func ReturnError(err error, line int) error {
	if err != nil {
		PrintError(err)
		errorString := "Error is " + err + " on line " + line
		return errors.New(errorString)
	}
}
