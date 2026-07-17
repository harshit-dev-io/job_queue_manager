package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
)

var validate = validator.New(validator.WithRequiredStructEnabled())
var cntxt context.Context = context.Background()
var redis_client *redis.Client

type Job struct{
	ID uuid.UUID					`json:"id"`
	TaskName string					`json:"task_name"`
	Payload json.RawMessage			`json:"payload"`
	MaxRetries int					`json:"max_retries"`
	Attempts int					`json:"attempts"`
}

type JobStatus string
const (
	StatusPending	JobStatus = "pending"
	StatusRunning	JobStatus = "Running"
	StatusCompleted	JobStatus = "completed"
	StatusFailed	JobStatus = "failed"
)

type Queue string
const (
	JobPending	Queue = "job:pending"
	JobProcessing	Queue = "job:processing"
	JobCompleted	Queue = "job:completed"
	JobFailed	Queue = "job:failed"
)
type Fields string
const (
	Data Fields = "data"
	Status Fields = "status"
	Message Fields = "message"
)

type AckMessage struct{
	ID uuid.UUID					`json:"id"`
	Status JobStatus				`json:"status"`
	CompletedAt time.Time			`json:"completed_at"`
}

type TaskFunc func(cntxt context.Context , payload json.RawMessage) error

var TaskRegistry = make(map[string]TaskFunc)

func RegisteredTask(){
	TaskRegistry["send_mail"] = SendEmail
}

func main(){

	err := godotenv.Load()
    if err != nil {
        err = godotenv.Load("../.env") 
        if err != nil {
            log.Println("Warning: Could not discover any .env file")
        }
    }

	redis_client = redis.NewClient(&redis.Options{
		Addr: os.Getenv("REDIS_ADDR"),
		DB: 0,
	})

	if err := redis_client.Ping(cntxt).Err() ; err != nil {
		log.Fatalf("failed to connect to redis : %v" , err)
	}

	LoadFromProcessing()

	go StartWorker()

	http.HandleFunc("/submit/" , AddTask)
	http.HandleFunc("/status/" , GetStatus)

	if err := http.ListenAndServe(":8080" , nil) ; err != nil {
		log.Print(err)
	}
	
}

func LoadFromProcessing(){
	var count int = 0

	for {
		_ , err := redis_client.BLMove(cntxt , string(JobProcessing) , string(JobPending) , "RIGHT" , "LEFT" , 1*time.Second).Result()
		if err == redis.Nil{
			break
		}else if err != nil{
			log.Print(err)
			break
		}
		count++
	}

	log.Printf("shifted %d tasks from processing to pending" , count)
}

func PushAckMessage(cntxt context.Context , id string , msg AckMessage) {
	msg_bytes , err := json.Marshal(msg)
	if err!=nil{
		log.Printf("error while converting ack message to bytes %v ", err)
	}	


		
	err = redis_client.HSet(cntxt , "job:"+id , string(Fields(Status)) , string(msg.Status) , string(Fields(Message)) , msg_bytes).Err()
	err = redis_client.LRem(cntxt , string(JobProcessing) , 1 , id).Err()
		
	if err != nil{
		if err == redis.Nil{
			log.Printf("redis error while performing push to redis %s queue" , msg.Status)
		}
		log.Printf("server error while performing push to redis %s queue" , msg.Status)
	}
	go CleanAckMessage(id)
}

func CleanAckMessage(id string){
	time.Sleep(30*time.Second)
	err := redis_client.Del(cntxt ,"job:"+id).Err()
	
	if err != nil{
		if err == redis.Nil{
			log.Printf("redis error while deleting %s" , id)
		}
		log.Printf("internal server error %v while deleting id  %s" , err , id)
	}
}

func AddTask(w http.ResponseWriter , r *http.Request){
	if r.Method != http.MethodPost{
		http.Error(w , "only post method allowed" , http.StatusMethodNotAllowed)
		return
	}
	var task struct{
		TaskName string 				`json:"task_name" validate:"required"`
		Payload json.RawMessage			`json:"payload" validate:"required"`
	}

	if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
		http.Error(w , "Invalid json payload" , http.StatusBadRequest)
		return
	}

	if err := validate.Struct(&task); err != nil {
		http.Error(w, "Missing required fields: 'task_name' and 'payload' are mandatory", http.StatusBadRequest)
		return 
	}

	var job Job = Job{
		ID: uuid.New(),
		TaskName: task.TaskName,
		Payload: task.Payload,
		MaxRetries: 3,
		Attempts: 0,
	}

	JobBytes , err := json.Marshal(job)

	if err != nil {
		http.Error(w , "Failed to encode job" , http.StatusInternalServerError)
		return 
	}

	err = redis_client.HSet(cntxt , "job:"+job.ID.String() , string(Fields(Data)) , JobBytes , string(Fields(Status)) , string(Queue(StatusPending))).Err()
	if err != nil {
		http.Error(w , "Failed to push job to queue " , http.StatusInternalServerError)
		return
	}
	err = redis_client.LPush(cntxt , string(Queue(JobPending)) , job.ID.String() ).Err()
	if err != nil {
		http.Error(w , "Failed to push job to queue" , http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type","application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{
		"job_id":job.ID.String(),
		"status":string(JobPending),
	})
}

func GetStatus(w http.ResponseWriter , r *http.Request){
	if r.Method != http.MethodGet {
		http.Error(w , "only Get method allowed" , http.StatusMethodNotAllowed)
		return
	}

	queryparams := r.URL.Query()
	id := queryparams.Get("id")
	
	status , err := redis_client.HGet(cntxt , "job:"+id , string(Fields(Status))).Result()
	
	if err != nil {
		if err == redis.Nil{
		http.Error(w , "not a valid key" , http.StatusBadRequest)
		return
	}
		http.Error(w , "error while fetching status" , http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type","application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":string(status),
	})
}