package mr

import (
    "fmt"
    "log"
    "net"
    "net/http"
    "net/rpc"
    "sync"
)

var void interface{}

type stringArray []string

type Coordinator struct {
    // filename list
    files        []string
    reduceId     int                 //
    midFilesMap  map[int]stringArray //
    midFilesList []int
    // 
    mapSend    map[string]interface{}
    reduceSend map[int]interface{} //key: reduceId
    nReduce    int
    ok         bool // 
    // 
    mtx sync.Mutex
}

func (c *Coordinator) AssignWorkerId(i *int, wi *WorkerInfo) error {
    c.mtx.Lock()
    wi.WorkId = c.reduceId
    wi.NReduce = c.nReduce
    c.reduceId++
    c.mtx.Unlock()
    return nil
}

// Your code here -- RPC handlers for the worker to call.

//
// an example RPC handler.
//
// the RPC argument and reply types are defined in rpc.go.
//
func (c *Coordinator) Example(args *ExampleArgs, reply *ExampleReply) error {
    reply.Y = args.X + 1
    return nil
}


func (c *Coordinator) AssignMapTask(req *MapRequest, resp *MapResponse) error {
    c.mtx.Lock()
    if len(c.files) == 0 {
        resp.Filename = ""
        if len(c.mapSend) == 0 {
            resp.State = "done"
            fmt.Println("map task done.")
        }
    } else {
        resp.Filename = c.files[0]
        c.files = c.files[1:]
        c.mapSend[resp.Filename] = void

    }
    c.mtx.Unlock()
    // fmt.Printf("c.files: %v\n", c.files)
    fmt.Printf("c.files: %v\n", c.files)
    fmt.Printf("c.mapSend: %v\n", c.mapSend)
    return nil
}


func (c *Coordinator) AssignReduceTask(req *ReduceRequest, resp *ReduceResponse) error {
    c.mtx.Lock()
    if len(c.midFilesList) == 0 {
        resp.Filenames = nil
        resp.ReduceId = -1
        if len(c.reduceSend) == 0 {
            resp.State = "done"
            fmt.Println("reduce task done")
        }
    } else {
        resp.ReduceId = c.midFilesList[0]
        c.midFilesList = c.midFilesList[1:]
        resp.Filenames = c.midFilesMap[resp.ReduceId]
        c.reduceSend[resp.ReduceId] = void
    }
    fmt.Printf("c.midFiles: %v\n", c.midFilesList)
    fmt.Printf("c.reduceSend: %v\n", c.reduceSend)
    c.mtx.Unlock()
    return nil
}

// 
// 
func (c *Coordinator) MapTaskResp(state *MapTaskState, resp *MapResponse) error {
    c.mtx.Lock()
    // 
    if state.State == "done" {
        // 
        delete(c.mapSend, state.Filename)

        for i := 0; i < c.nReduce; i++ {
            name := fmt.Sprintf("%s-%d-%d_%d", "mr-mid", state.WorkerId, state.TaskId, i)
            _, ok := c.midFilesMap[i]
            if !ok {
                c.midFilesMap[i] = stringArray{}
            }
            c.midFilesMap[i] = append(c.midFilesMap[i], name)
        }

        // fmt.Printf("names[0]: %v\n", names[0])
    } else {
        
        c.files = append(c.files, state.Filename)
        
    }
    c.mtx.Unlock()
    return nil
}

func (c *Coordinator) ReduceStateResp(state *ReduceTaskState, resp *ReduceResponse) error {
    c.mtx.Lock()
    // fmt.Printf("state: %v\n", state)
    if state.State == "done" {
        // 
        delete(c.reduceSend, state.ReduceId)
        // fmt.Printf("c.reduceSend: %v\n", c.reduceSend)
        if len(c.reduceSend) == 0 && len(c.midFilesList) == 0 && !c.ok {
            c.ok = true
        }
    } else {
        // 
        c.midFilesList = append(c.midFilesList, state.ReduceId)
        // 
        // 
    }
    c.mtx.Unlock()

    

    // fmt.Printf("c.midfiles: %v\n", c.midFiles)
    return nil
}


func (c *Coordinator) server() {
    rpc.Register(c)
    rpc.HandleHTTP()
    l, e := net.Listen("tcp", ":1234")
    // sockname := coordinatorSock()
    // os.Remove(sockname)
    // l, e := net.Listen("unix", sockname)
    if e != nil {
        log.Fatal("listen error:", e)
    }
    go http.Serve(l, nil)
}

//
// main/mrcoordinator.go calls Done() periodically to find out
// if the entire job has finished.
//
func (c *Coordinator) Done() bool {
    c.mtx.Lock()
    ret := c.ok
    c.mtx.Unlock()

    
    return ret
}

//
// create a Coordinator.
// main/mrcoordinator.go calls this function.
// nReduce is the number of reduce tasks to use.
//
func MakeCoordinator(files []string, nReduce int) *Coordinator {
    c := Coordinator{}

    // Your code here.
    c.nReduce = nReduce
    c.files = files
    c.midFilesMap = map[int]stringArray{}
    c.midFilesList = []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
    c.mtx = sync.Mutex{}
    c.mapSend = make(map[string]interface{})
    c.reduceSend = make(map[int]interface{})
    c.server()
    fmt.Printf("c: %v\n", c)
    return &c
}
