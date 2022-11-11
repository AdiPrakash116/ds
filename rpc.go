package mr

//
// RPC definitions.
//
// remember to capitalize all names.
//

import (
    "os"
    "strconv"
)

//
// example to show how to declare the arguments
// and reply for an RPC.
//

type ExampleArgs struct {
    X int
}

type ExampleReply struct {
    Y int
}

// Add your RPC definitions here.

// Cook up a unique-ish UNIX-domain socket name
// in /var/tmp, for the coordinator.
// Can't use the current directory since
// Athena AFS doesn't support UNIX-domain sockets.
func coordinatorSock() string {
    s := "/var/tmp/824-mr-"
    s += strconv.Itoa(os.Getuid())
    return s
}


type MapRequest struct {
}


type MapResponse struct {
    Filename string
    State    string
}

type MapTaskState struct {
    Filename string
    WorkerId int
    TaskId   int
    State    string
}

type ReduceRequest struct {
}

type ReduceResponse struct {
    ReduceId  int
    Filenames []string
    State     string
}

type ReduceTaskState struct {
    ReduceId int
    State    string
}

type WorkerInfo struct {
    NReduce int
    WorkId  int
}