package mr

import (
    "encoding/json"
    "fmt"
    "hash/fnv"
    "io"
    "io/ioutil"
    "log"
    "net/rpc"
    "os"
    "sort"
    "time"
)

//
// Map functions return a slice of KeyValue.
//
type KeyValue struct {
    Key   string
    Value string
}

//
// use ihash(key) % NReduce to choose the reduce
// task number for each KeyValue emitted by Map.
//
func ihash(key string) int {
    h := fnv.New32a()
    h.Write([]byte(key))
    return int(h.Sum32() & 0x7fffffff)
}

var workerInfo WorkerInfo
var midFiles []*os.File

func WorkerMap(mapf func(string, string) []KeyValue, c *rpc.Client) {
    
    retryTimes := 0
    taskid := 0

    for retryTimes < 3 {

        req := MapRequest{}
        resp := MapResponse{}
        err2 := c.Call("Coordinator.AssignMapTask", &req, &resp)
        if err2 != nil {
            fmt.Printf("err2: %v\n", err2)
            time.Sleep(time.Second)
            retryTimes++
            continue
        }
        retryTimes = 0
        if resp.State == "done" {
            fmt.Printf("worker %v map work done.", workerInfo.WorkId)
            return
        }
        if resp.Filename == "" {
            
            fmt.Println("map job assign done. but others workers are running")
            time.Sleep(time.Second)
            continue
        }

        

        req2 := MapTaskState{resp.Filename, workerInfo.WorkId, taskid, "done"}
        resp2 := MapResponse{}

        f, err := os.Open(resp.Filename)
        if err != nil {
            log.Fatal(err)
            req2.State = "nosuchfile"
            c.Call("Coordinator.MapTaskResp", &req2, &resp2)
            continue
        }
        defer f.Close()
        content, err := ioutil.ReadAll(f)
        if err != nil {
            log.Fatalf("cannot read %v", resp.Filename)
            req2.State = "filereaderr"
            c.Call("Coordinator.MapTaskResp", &req2, &resp2)
        }

        kvs := mapf(resp.Filename, string(content))

        // enc := json.NewEncoder(out)

        encs := []*json.Encoder{}

        midFiles = []*os.File{}
        
        for i := 0; i < workerInfo.NReduce; i++ {
            f, _ := os.CreateTemp("", "di-mp")
            midFiles = append(midFiles, f)
        }

        for i := 0; i < workerInfo.NReduce; i++ {
            encs = append(encs, json.NewEncoder(midFiles[i]))
        }

        for _, kv := range kvs {
            encs[ihash(kv.Key)%workerInfo.NReduce].Encode(kv)
        }

        for i := 0; i < workerInfo.NReduce; i++ {
            name := fmt.Sprintf("%s-%d-%d_%d", "mr-mid", workerInfo.WorkId, taskid, i)
            os.Rename(midFiles[i].Name(), name)
        }

        req2.Filename = resp.Filename
        req2.TaskId = taskid
        req2.State = "done"
        err = c.Call("Coordinator.MapTaskResp", &req2, &resp2)
        if err != nil {
            fmt.Printf("err: %v\n", err)
        }
        taskid++
        // then send to MapTaskResp
        // fmt.Printf("resp: %v\n", resp)

    }
}

func WorkerReduce(reducef func(string, []string) string, client *rpc.Client) {
    

RESTARTREDUCE:

    retryTimes := 0
    for retryTimes < 3 {
        req := ReduceRequest{}
        var resp ReduceResponse
        err2 := client.Call("Coordinator.AssignReduceTask", &req, &resp)

        if err2 != nil {
            fmt.Printf("err2: %v\n", err2)
            time.Sleep(time.Second)
            retryTimes++
            continue
        }

        retryTimes = 0
        if resp.State == "done" {
            return
        }
        if resp.ReduceId == -1 {
            time.Sleep(time.Second)
            fmt.Println("reduce job assign done. but others workers are running")
            continue
        }

        
        req2 := ReduceTaskState{resp.ReduceId, ""}
        resp2 := ReduceResponse{}
        resp2.ReduceId = resp.ReduceId
        fmt.Printf("worker id %v: %v\n", workerInfo.WorkId, resp.Filenames)
        reduceId := resp2.ReduceId
        name := fmt.Sprintf("%s-%d", "mr-out", reduceId)

        outtmpfile, _ := os.CreateTemp("", "di-out")

        // outFile, _ = os.OpenFile(name, os.O_WRONLY|os.O_APPEND, 0666)

        kva := []KeyValue{}
        jsonParseState := true

        for _, filename := range resp.Filenames {
            f, err := os.Open(filename)
            // fmt.Printf("resp.Filename: %v\n", resp.Filename)
            defer f.Close()
            if err != nil {
                fmt.Println("mid file open wrong:", err)
                req2.State = "nosuchfile"
                client.Call("Coordinator.ReduceStateResp", &req2, &resp2)
                goto RESTARTREDUCE
            }
            d := json.NewDecoder(f)

            for {
                var kv KeyValue
                if err := d.Decode(&kv); err != nil {
                    if err == io.EOF {
                        break
                    }
                    fmt.Println("json parse:", err)
                    req2.State = "jsonparseerr"
                    client.Call("Coordinator.ReduceStateResp", &req2, &resp2)
                    jsonParseState = false
                    break
                }
                // fmt.Printf("kv: %v\n", kv)
                kva = append(kva, kv)
            }
        }

        if jsonParseState {
            sort.Sort(byKey(kva))

            i := 0
            for i < len(kva) {
                j := i + 1
                for j < len(kva) && kva[i].Key == kva[j].Key {
                    j++
                }
                vv := []string{}
                for k := i; k < j; k++ {
                    vv = append(vv, kva[k].Value)
                }
                s := reducef(kva[i].Key, vv)
                
                fmt.Fprintf(outtmpfile, "%v %v\n", kva[i].Key, s)
                // fmt.Fprintf(outFiles[ihash(kva[i].Key)%workerInfo.NReduce], "%v %v\n", kva[i].Key, s)
                i = j
            }

        } else {
            goto RESTARTREDUCE
        }

        req2.State = "done"
        req2.ReduceId = resp.ReduceId
        err := client.Call("Coordinator.ReduceStateResp", &req2, &resp2)
        if err == nil {
            os.Rename(outtmpfile.Name(), name)
        }

    }
}

type byKey []KeyValue

func (a byKey) Len() int           { return len(a) }
func (a byKey) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byKey) Less(i, j int) bool { return a[i].Key < a[j].Key }

//
// main/mrworker.go calls this function.
//
func Worker(mapf func(string, string) []KeyValue,
    reducef func(string, []string) string) {

    // 
    i := 0
    c, _ := rpc.DialHTTP("tcp", "127.0.0.1"+":1234")
    defer c.Close()

    c.Call("Coordinator.AssignWorkerId", &i, &workerInfo)

    time.Sleep(time.Second)
    WorkerMap(mapf, c)

    
    WorkerReduce(reducef, c)
}

//
// example function to show how to make an RPC call to the coordinator.
//
// the RPC argument and reply types are defined in rpc.go.
//
func CallExample() {

    // declare an argument structure.
    args := ExampleArgs{}

    // fill in the argument(s).
    args.X = 99

    // declare a reply structure.
    reply := ExampleReply{}

    // send the RPC request, wait for the reply.
    call("Coordinator.Example", &args, &reply)

    // reply.Y should be 100.
    fmt.Printf("reply.Y %v\n", reply.Y)
}

func clearMapMidFiles(midFiles []*os.File) {
    for _, f := range midFiles {
        f.Close()
        os.Remove(f.Name())
    }
}

//
// send an RPC request to the coordinator, wait for the response.
// usually returns true.
// returns false if something goes wrong.
//
func call(rpcname string, args interface{}, reply interface{}) bool {
    c, err := rpc.DialHTTP("tcp", "127.0.0.1"+":1234")
    // sockname := coordinatorSock()
    // c, err := rpc.DialHTTP("unix", sockname)
    if err != nil {
        log.Fatal("dialing:", err)
    }
    defer c.Close()

    err = c.Call(rpcname, args, reply)
    if err == nil {
        return true
    }

    fmt.Println(err)
    return false
}
