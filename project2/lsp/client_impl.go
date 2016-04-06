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

	oldestAckInWindow	int
	oldestUnAckData		int
	seqNumToSend		int
							// next one to the next msg we send
	expectedSN		int		// the next expected SN

	connectChan		(chan Message)

	epochCh			(chan int)
	numEpochs 		int
	epochLimit 		int
	epochMillis 	int

	windowSize 		int
	ackWindow		map[int]Message
	dataWindow 		map[int]Message

	intermediateToRead (chan Message)
	toRead			(chan Message)
	toWrite			(chan Message)

	lostConn		(chan int)
	closed 			(chan int)
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
		connID:         0,
		conn:			conn,
		connectChan:	make(chan *Message,1),

		seqNumToSend:	0,			// next Sn to send
		expectedSN:		0,

		epochCh: 		make(chan int, 1),
		numEpochs: 		0,
		epochLimit: 	params.EpochLimit,
		epochMillis:	params.EpochMillis,

		windowSize: 	params.WindowSize,
		ackWindow:		make(map[int]Message),
		dataWindow:		make(map[int]Message),

		intermediateToRead: make(chan *Message, 1),
		toRead:			make(chan *Message, 1),
		toWrite:		make(chan *Message, 1),

		lostConn:		make(chan int, 1),
		closed:			make(chan int, 1),
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
	return c.id
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
	return errors.New("not yet implemented")
}

func (c *client) Close() error {
	return errors.New("not yet implemented")
}

// ===================== HANDLING FUNCTIONS ================================

func goClient(c *client) {
	epochCh = time.NewTicker(c.epochMillis).C
	for {
		select {
		case <- epochCh:
			c.epoch(c)
		// if at any time there is a read message to process
		case msg <- intermediateToRead:
			c.readHelper(msg)
		}
	}
}

// always reading from the connection
func goRead(c *client) {
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

func (c *client) readHelper(msg Message) {
	select {
	// If we receive a data message, send an ack back
		// Make sure to add it to the ackWindow so we can resend acks during epoch
	case msg.Type == MsgData:
		// Make sure there is a connection
		if c.connID > 0 {
			maxAcceptableSN := c.expectedSN + c.windowSize
			// If not a duplicate, insert into toRead channel for Read to work with
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
		// If this is an ack for a data message
			// If this ack is for a message in our window of unacked data, delete the msg from the unacked data msgs window and shift everything over
			if msg.SeqNum == c.oldestUnAckData {
				c.oldestUnAckData = msg.SeqNum
			}
			c.moveDataWindow(msg)
		}

		// Move our data window
	default:
	}
}

func (c *client) moveDataWindow(msg Message) {
	// Delete the data msg that was acked from the dataWindow
	for index,curr_msg := c.dataWindow {
		if curr_msg.SeqNum == msg.SeqNum {
			c.dataWindow[index].occupiedSlot = 0
			c.dataWindow[index].message = nil
		}
	}
	// Shift everything over
	i := 0
	for (i < 4) {

		i++
	}
}

func epoch(msg Message) {
	// actually do everything that's supposed to happen during the epoch
}

// ===================== HELPER FUNCTIONS ================================

func connectionRequest() (c *client) {
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
	PrintError(received_err)
	received_msg := Message{}
	json.Unmarshal(buff[0:num_bytes], &received_msg)
	return received_msg
}

func (c *client) moveAckWindow(msg Message) {
	// We just sent an ack, so we have to update the most recently sent Acks
	numMsgs := len(c.ackWindow)
	index := 0
	for index, _ := range c.ackWindow {
		// If we are not yet at the last one and the next one is actual data, move it
		if (index < 4) && (c.ackWindow[index].occupiedSlot == 1){
			c.ackWindow[index] = c.ackWindow[index+1]
		// Else we hit the last index or an unused slot, insert the curent msg there
		} else {
			current_ackStruct := {
									occupiedSlot: 1,
									message: msg,
								}
			c.ackWindow[index] = current_ackStruct
		}
	}
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
