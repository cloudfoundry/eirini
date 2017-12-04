# Task Examples

## Desiring a Task

```go
client := bbs.NewClient("http://10.244.16.2:8889")
err = client.DesireTask(
  "some-guid",
  "some-domain",
  &models.TaskDefinition{
    RootFs: "docker:///busybox",
    DiskMb:   1024,
    MemoryMb: 1024,
    MaxPids:  1024,
    CpuWeight: 42,
    Action: models.WrapAction(&models.RunAction{
      User:           "root",
      Path:           "sh",
      Args:           []string{"-c", "echo hello world > result-file.txt"},
      ResourceLimits: &models.ResourceLimits{},
    }),
    CompletionCallbackUrl: "http://10.244.16.6:7890",
    ResultFile:            "result-file.txt",
  },
)
```

## Polling for Task Info

```go
for {
  task, err := client.TaskByGuid("some-guid")
  if err != nil {
    log.Printf("failed to fetch task!")
    panic(err)
  }
  if task.State == models.Task_Resolving {
    log.Printf("here's the result from your polled task:\n %s\n\n", task.Result)
    break
  }
  time.Sleep(time.Second)
}
```

## Recieving a TaskCallbackResponse

To receive the TaskCallbackResponse, we first start an HTTP server.

```go
func taskCallbackHandler(w http.ResponseWriter, r *http.Request) {
	var taskResponse models.TaskCallbackResponse
	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Printf("failed to read task response")
		panic(err)
	}
	err = json.Unmarshal(data, &taskResponse)
	if err != nil {
		log.Printf("failed to unmarshal json")
		panic(err)
	}
	log.Printf("here's the result from your TaskCallbackResponse:\n %s\n\n", taskResponse.Result)
}

http.HandleFunc("/", taskCallbackHandler)
go http.ListenAndServe("7890", nil)
```

Suppose this server is running on IP `10.244.16.6`. When the above task is desired, it will run on Diego, echo 'hello world' to the file `result-file.txt`, and complete successfully. Diego will then POST a JSON-encoded `TaskCallbackResponse` to the server. The `Result` field of the `TaskCallbackResponse` will be the contents of the `result-file.txt` file, namely 'hello world'.


## Cancelling a Task

```go
client := bbs.NewClient(url)
err := client.CancelTask("some-guid")
if err != nil {
  log.Printf("failed to cancel task: " + err.Error())
}
```

[back](README.md)
