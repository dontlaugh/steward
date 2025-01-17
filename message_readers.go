package steward

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// readStartupFolder will check the <workdir>/startup folder when Steward
// starts for messages to process.
// The purpose of the startup folder is that we can define messages on a
// node that will be run when Steward starts up.
// Messages defined in the startup folder should have the toNode set to
// self, and the from node set to where we want the answer sent. The reason
// for this is that all replies normally pick up the host from the original
// first message, but here we inject it on an end node so we need to specify
// the fromNode to get the reply back to the node we want.
//
// Messages read from the startup folder will be directly called by the handler
// locally, and the message will not be sent via the nats-server.
func (s *server) readStartupFolder() {

	// Get the names of all the files in the startup folder.
	const startupFolder = "startup"
	filePaths, err := s.getFilePaths(startupFolder)
	if err != nil {
		er := fmt.Errorf("error: readStartupFolder: unable to get filenames: %v", err)
		s.errorKernel.errSend(s.processInitial, Message{}, er)
		return
	}

	for _, filePath := range filePaths {

		// Read the content of each file.
		readBytes, err := func(filePath string) ([]byte, error) {
			fh, err := os.Open(filePath)
			if err != nil {
				er := fmt.Errorf("error: failed to open file in startup folder: %v", err)
				return nil, er
			}
			defer fh.Close()

			b, err := io.ReadAll(fh)
			if err != nil {
				er := fmt.Errorf("error: failed to read file in startup folder: %v", err)
				return nil, er
			}

			return b, nil
		}(filePath)

		if err != nil {
			s.errorKernel.errSend(s.processInitial, Message{}, err)
			continue
		}

		readBytes = bytes.Trim(readBytes, "\x00")

		// unmarshal the JSON into a struct
		sams, err := s.convertBytesToSAMs(readBytes)
		if err != nil {
			er := fmt.Errorf("error: startup folder: malformed json read: %v", err)
			s.errorKernel.errSend(s.processInitial, Message{}, er)
			continue
		}

		// Check if fromNode field is specified, and remove the message if blank.
		for i := range sams {
			if sams[i].Message.FromNode == "" {
				sams = append(sams[:i], sams[i+1:]...)
				er := fmt.Errorf(" error: missing from field in startup message")
				s.errorKernel.errSend(s.processInitial, Message{}, er)
			}

			// Bounds check.
			if i == len(sams)-1 {
				break
			}
		}

		// Send the SAM struct to be picked up by the ring buffer.
		// s.ringBufferBulkInCh <- sams

		// ---

		// Range over all the sams, find the process, check if the method exists, and
		// handle the message by starting the correct method handler.
		for i := range sams {
			processName := processNameGet(sams[i].Subject.name(), processKindSubscriber)

			s.processes.active.mu.Lock()
			p := s.processes.active.procNames[processName]
			s.processes.active.mu.Unlock()

			mh, ok := p.methodsAvailable.CheckIfExists(sams[i].Message.Method)
			if !ok {
				er := fmt.Errorf("error: subscriberHandler: method type not available: %v", p.subject.Event)
				p.errorKernel.errSend(p, sams[i].Message, er)
				continue
			}

			_, err = mh.handler(p, sams[i].Message, s.nodeName)
			if err != nil {
				er := fmt.Errorf("error: subscriberHandler: handler method failed: %v", err)
				p.errorKernel.errSend(p, sams[i].Message, er)
				continue
			}
		}

	}
}

// getFilePaths will get the names of all the messages in
// the folder specified from current working directory.
func (s *server) getFilePaths(dirName string) ([]string, error) {
	dirPath, err := os.Executable()
	dirPath = filepath.Dir(dirPath)
	fmt.Printf(" * DEBUG: dirPath=%v\n", dirPath)
	if err != nil {
		return nil, fmt.Errorf("error: startup folder: unable to get the working directory %v: %v", dirPath, err)
	}

	dirPath = filepath.Join(dirPath, dirName)

	// Check if the startup folder exist.
	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		err := os.MkdirAll(dirPath, 0700)
		if err != nil {
			er := fmt.Errorf("error: failed to create startup folder: %v", err)
			return nil, er
		}
	}

	fInfo, err := ioutil.ReadDir(dirPath)
	if err != nil {
		er := fmt.Errorf("error: failed to get filenames in startup folder: %v", err)
		return nil, er
	}

	filePaths := []string{}

	for _, v := range fInfo {
		realpath := filepath.Join(dirPath, v.Name())
		filePaths = append(filePaths, realpath)
	}

	return filePaths, nil
}

// readSocket will read the .sock file specified.
// It will take a channel of []byte as input, and it is in this
// channel the content of a file that has changed is returned.
func (s *server) readSocket() {
	// Loop, and wait for new connections.
	for {
		conn, err := s.StewardSocket.Accept()
		if err != nil {
			er := fmt.Errorf("error: failed to accept conn on socket: %v", err)
			s.errorKernel.errSend(s.processInitial, Message{}, er)
		}

		go func(conn net.Conn) {
			defer conn.Close()

			var readBytes []byte

			for {
				b := make([]byte, 1500)
				_, err = conn.Read(b)
				if err != nil && err != io.EOF {
					er := fmt.Errorf("error: failed to read data from socket: %v", err)
					s.errorKernel.errSend(s.processInitial, Message{}, er)
					return
				}

				readBytes = append(readBytes, b...)

				if err == io.EOF {
					break
				}
			}

			readBytes = bytes.Trim(readBytes, "\x00")

			// unmarshal the JSON into a struct
			sams, err := s.convertBytesToSAMs(readBytes)
			if err != nil {
				er := fmt.Errorf("error: malformed json received on socket: %s\n %v", readBytes, err)
				s.errorKernel.errSend(s.processInitial, Message{}, er)
				return
			}

			for i := range sams {

				// Fill in the value for the FromNode field, so the receiver
				// can check this field to know where it came from.
				sams[i].Message.FromNode = Node(s.nodeName)

				// Send an info message to the central about the message picked
				// for auditing.
				er := fmt.Errorf("info: message read from socket on %v: %v", s.nodeName, sams[i].Message)
				s.errorKernel.errSend(s.processInitial, Message{}, er)
			}

			// Send the SAM struct to be picked up by the ring buffer.
			s.toRingBufferCh <- sams

		}(conn)
	}
}

// readTCPListener wait and read messages delivered on the TCP
// port if started.
// It will take a channel of []byte as input, and it is in this
// channel the content of a file that has changed is returned.
func (s *server) readTCPListener() {
	ln, err := net.Listen("tcp", s.configuration.TCPListener)
	if err != nil {
		log.Printf("error: readTCPListener: failed to start tcp listener: %v\n", err)
		os.Exit(1)
	}
	// Loop, and wait for new connections.
	for {

		conn, err := ln.Accept()
		if err != nil {
			er := fmt.Errorf("error: failed to accept conn on socket: %v", err)
			s.errorKernel.errSend(s.processInitial, Message{}, er)
			continue
		}

		go func(conn net.Conn) {
			defer conn.Close()

			var readBytes []byte

			for {
				b := make([]byte, 1500)
				_, err = conn.Read(b)
				if err != nil && err != io.EOF {
					er := fmt.Errorf("error: failed to read data from tcp listener: %v", err)
					s.errorKernel.errSend(s.processInitial, Message{}, er)
					return
				}

				readBytes = append(readBytes, b...)

				if err == io.EOF {
					break
				}
			}

			readBytes = bytes.Trim(readBytes, "\x00")

			// unmarshal the JSON into a struct
			sam, err := s.convertBytesToSAMs(readBytes)
			if err != nil {
				er := fmt.Errorf("error: malformed json received on tcp listener: %v", err)
				s.errorKernel.errSend(s.processInitial, Message{}, er)
				return
			}

			for i := range sam {

				// Fill in the value for the FromNode field, so the receiver
				// can check this field to know where it came from.
				sam[i].Message.FromNode = Node(s.nodeName)
			}

			// Send the SAM struct to be picked up by the ring buffer.
			s.toRingBufferCh <- sam

		}(conn)
	}
}

func (s *server) readHTTPlistenerHandler(w http.ResponseWriter, r *http.Request) {

	var readBytes []byte

	for {
		b := make([]byte, 1500)
		_, err := r.Body.Read(b)
		if err != nil && err != io.EOF {
			er := fmt.Errorf("error: failed to read data from tcp listener: %v", err)
			s.errorKernel.errSend(s.processInitial, Message{}, er)
			return
		}

		readBytes = append(readBytes, b...)

		if err == io.EOF {
			break
		}
	}

	readBytes = bytes.Trim(readBytes, "\x00")

	// unmarshal the JSON into a struct
	sam, err := s.convertBytesToSAMs(readBytes)
	if err != nil {
		er := fmt.Errorf("error: malformed json received on HTTPListener: %v", err)
		s.errorKernel.errSend(s.processInitial, Message{}, er)
		return
	}

	for i := range sam {

		// Fill in the value for the FromNode field, so the receiver
		// can check this field to know where it came from.
		sam[i].Message.FromNode = Node(s.nodeName)
	}

	// Send the SAM struct to be picked up by the ring buffer.
	s.toRingBufferCh <- sam

}

func (s *server) readHttpListener() {
	go func() {
		n, err := net.Listen("tcp", s.configuration.HTTPListener)
		if err != nil {
			log.Printf("error: startMetrics: failed to open prometheus listen port: %v\n", err)
			os.Exit(1)
		}
		mux := http.NewServeMux()
		mux.HandleFunc("/", s.readHTTPlistenerHandler)

		err = http.Serve(n, mux)
		if err != nil {
			log.Printf("error: startMetrics: failed to start http.Serve: %v\n", err)
			os.Exit(1)
		}
	}()
}

// The subject are made up of different parts of the message field.
// To make things easier and to avoid figuring out what the subject
// is in all places we've created the concept of subjectAndMessage
// (sam) where we get the subject for the message once, and use the
// sam structure with subject alongside the message instead.
type subjectAndMessage struct {
	Subject `json:"subject" yaml:"subject"`
	Message `json:"message" yaml:"message"`
}

// convertBytesToSAMs will range over the  byte representing a message given in
// json format. For each element found the Message type will be converted into
// a SubjectAndMessage type value and appended to a slice, and the slice is
// returned to the caller.
func (s *server) convertBytesToSAMs(b []byte) ([]subjectAndMessage, error) {
	MsgSlice := []Message{}

	err := yaml.Unmarshal(b, &MsgSlice)
	if err != nil {
		return nil, fmt.Errorf("error: unmarshal of file failed: %#v", err)
	}

	// Check for toNode and toNodes field.
	MsgSlice = s.checkMessageToNodes(MsgSlice)
	s.metrics.promUserMessagesTotal.Add(float64(len(MsgSlice)))

	sam := []subjectAndMessage{}

	// Range over all the messages parsed from json, and create a subject for
	// each message.
	for _, m := range MsgSlice {
		sm, err := newSubjectAndMessage(m)
		if err != nil {
			er := fmt.Errorf("error: newSubjectAndMessage: %v", err)
			s.errorKernel.errSend(s.processInitial, m, er)

			continue
		}
		sam = append(sam, sm)
	}

	return sam, nil
}

// checkMessageToNodes will check that either toHost or toHosts are
// specified in the message. If not specified it will drop the message
// and send an error.
// if toNodes is specified, the original message will be used, and
// and an individual message will be created with a toNode field for
// each if the toNodes specified.
func (s *server) checkMessageToNodes(MsgSlice []Message) []Message {
	msgs := []Message{}

	for _, v := range MsgSlice {
		switch {
		// if toNode specified, we don't care about the toHosts.
		case v.ToNode != "":
			msgs = append(msgs, v)
			continue

		// if toNodes specified, we use the original message, and
		// create new node messages for each of the nodes specified.
		case len(v.ToNodes) != 0:
			for _, n := range v.ToNodes {
				m := v
				// Set the toNodes field to nil since we're creating
				// an individual toNode message for each of the toNodes
				// found, and hence we no longer need that field.
				m.ToNodes = nil
				m.ToNode = n
				msgs = append(msgs, m)
			}
			continue

		// No toNode or toNodes specified. Drop the message by not appending it to
		// the slice since it is not valid.
		default:
			er := fmt.Errorf("error: no toNode or toNodes where specified in the message, dropping message: %v", v)
			s.errorKernel.errSend(s.processInitial, v, er)
			continue
		}
	}

	return msgs
}

// newSubjectAndMessage will look up the correct values and value types to
// be used in a subject for a Message (sam), and return the a combined structure
// of type subjectAndMessage.
func newSubjectAndMessage(m Message) (subjectAndMessage, error) {
	// We need to create a tempory method type to look up the kind for the
	// real method for the message.
	var mt Method

	tmpH := mt.getHandler(m.Method)
	if tmpH == nil {
		return subjectAndMessage{}, fmt.Errorf("error: newSubjectAndMessage: no such request type defined: %v", m.Method)
	}

	switch {
	case m.ToNode == "":
		return subjectAndMessage{}, fmt.Errorf("error: newSubjectAndMessage: ToNode empty: %+v", m)
	case m.Method == "":
		return subjectAndMessage{}, fmt.Errorf("error: newSubjectAndMessage: Method empty: %v", m)
	}

	sub := Subject{
		ToNode: string(m.ToNode),
		Event:  tmpH.getKind(),
		Method: m.Method,
	}

	sam := subjectAndMessage{
		Subject: sub,
		Message: m,
	}

	return sam, nil
}
