package main

import (
	"github.com/couchbase/gocbcore/v10"
	"sync"
)

type Stream interface {
	Start()
	Wait()
	SaveCheckpoint()
	Stop()
	AddListener(listener Listener)
}

type stream struct {
	client          Client
	finishedStreams sync.WaitGroup
	listeners       []Listener
	streamsLock     sync.Mutex
	streams         []uint16
	checkpoint      Checkpoint
	Metadata        Metadata
}

func (s *stream) Listener(event int, data interface{}, err error) {
	if err != nil {
		return
	}

	if event == EndName {
		s.finishedStreams.Done()
	}

	if IsMetadata(data) {
		return
	}

	for _, listener := range s.listeners {
		listener(event, data, err)
	}
}

func (s *stream) Start() {
	vbIds := s.client.GetMembership().GetVBuckets()
	vBucketNumber := len(vbIds)

	observer := NewObserver(vbIds, s.Listener)

	failoverLogs, err := s.client.GetFailoverLogs(vbIds)

	if err != nil {
		panic(err)
	}

	s.finishedStreams.Add(vBucketNumber)

	var openWg sync.WaitGroup
	openWg.Add(vBucketNumber)

	s.checkpoint = NewCheckpoint(observer, vbIds, failoverLogs, s.client.GetBucketUuid(), s.Metadata)
	observerState := s.checkpoint.Load(s.client.GetGroupName())

	for _, vbId := range vbIds {
		go func(innerVbId uint16) {
			ch := make(chan error)

			err := s.client.OpenStream(
				innerVbId,
				failoverLogs[innerVbId].VbUUID,
				observerState[innerVbId],
				observer,
				func(entries []gocbcore.FailoverEntry, err error) {
					ch <- err
				},
			)

			err = <-ch

			if err != nil {
				panic(err)
			}

			s.streamsLock.Lock()
			defer s.streamsLock.Unlock()

			s.streams = append(s.streams, innerVbId)

			openWg.Done()
		}(vbId)
	}

	openWg.Wait()
}

func (s *stream) Wait() {
	s.finishedStreams.Wait()
	s.SaveCheckpoint()
}

func (s *stream) SaveCheckpoint() {
	s.checkpoint.Save(s.client.GetGroupName())
}

func (s *stream) Stop() {
	for _, stream := range s.streams {
		ch := make(chan error)

		err := s.client.CloseStream(stream, func(err error) {
			ch <- err
		})

		err = <-ch

		if err != nil {
			panic(err)
		}

		s.finishedStreams.Done()
	}
}

func (s *stream) AddListener(listener Listener) {
	s.listeners = append(s.listeners, listener)
}

func NewStream(client Client, metadata Metadata) Stream {
	return &stream{
		client:          client,
		finishedStreams: sync.WaitGroup{},
		listeners:       []Listener{},
		Metadata:        metadata,
	}
}

func NewStreamWithListener(client Client, metadata Metadata, listener Listener) Stream {
	return &stream{
		client:          client,
		finishedStreams: sync.WaitGroup{},
		listeners:       []Listener{listener},
		Metadata:        metadata,
	}
}
