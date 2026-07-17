package main

import (
	"time"
	"github.com/redis/go-redis/v9"
	"encoding/json"
	"log"
)

func StartWorker(){
	RegisteredTask()
	for {
		JobId , err := redis_client.BLMove(
			cntxt,
			string(JobPending),
			string(JobProcessing),
			"RIGHT",
			"LEFT",
			10*time.Second,
		).Result()
		if err != nil {
			if err == redis.Nil{
				time.Sleep(2*time.Second)
				continue
				} 
			log.Printf("error while performing BlMove %v" , err)
			time.Sleep(2*time.Second)
			continue
			}

		JobBytes , err := redis_client.HGet(cntxt , "job:"+JobId , string(Fields(Data))).Result()
		err = redis_client.HSet(cntxt , "job:"+JobId , string(Fields(Status)) , string(Queue(JobProcessing))).Err()
			
		var job Job
		err = json.Unmarshal([]byte(JobBytes) , &job)
		if err != nil {
			log.Printf("error while performing unmarshel job %v" , err)
			time.Sleep(2*time.Second)
			continue
		}
		
		TaskExecutor , exists := TaskRegistry[job.TaskName]
		if !exists {
			log.Printf("error invalid taskname  %s" , job.TaskName)
			time.Sleep(2*time.Second)
			continue
		}

		var msg AckMessage
		err  = TaskExecutor(cntxt , job.Payload)
		if err != nil{
			msg = AckMessage{
				ID: job.ID,
				Status: JobStatus(StatusFailed),
				CompletedAt: time.Now(),
			}
		}else{
			msg = AckMessage{
				ID: job.ID , 
				Status: JobStatus(StatusCompleted),
				CompletedAt: time.Now(),
			}
		}

		PushAckMessage(cntxt , job.ID.String() , msg)
	}
}
