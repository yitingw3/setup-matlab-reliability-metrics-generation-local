// This script is to get the reliability of setup-matlab action v1 v.s. v2-beta
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"setup-matlab-generate-matrix-local/chart"
	"time"

	"github.com/go-echarts/go-echarts/v2/components"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

type GithubActionRunsResponse struct {
	TotalCount   int `json:"total_count"`
	WorkflowRuns []struct {
		JobsUrl string `json:"jobs_url"`
		// You can add more fields here if needed
	} `json:"workflow_runs"`
}

type GithubActionJobsResponse struct {
	TotalCount int   `json:"total_count"`
	Jobs       []Job `json:"jobs"`
}

type Job struct {
	ID            int      `json:"id"`
	Name          string   `json:"name"`
	Labels        []string `json:"labels"`
	Status        string   `json:"status"`
	Conclusion    string   `json:"conclusion"`
	StartTime     string   `json:"started_at"`
	CompletedTime string   `json:"completed_at"`
	RunTime       int64    ``
	Steps         []Step   `json:"steps"`
}

type Step struct {
	Name          string `json:"name"`
	Status        string `json:"status"`
	Conclusion    string `json:"conclusion"`
	StartTime     string `json:"started_at"`
	CompletedTime string `json:"completed_at"`
	RunTime       int64  ``
}

var verNums = []string{"v1", "v2-beta"}
var platforms = []string{"ubuntu-22.04", "macos-12", "windows-2022"}

var runsRes GithubActionRunsResponse
var jobsRes GithubActionJobsResponse
var clientDB *mongo.Client
var dbName = "setupMatlabPerfDB"

func main() {
	//return the specific url for the latest workflowrun, containing the workflow_id
	getWorkflowRunsData()
	//Initialize a database connection
	clientDB, ctx := connectDB()
	//If existing outdated data, remove them
	deleteOldData(clientDB, ctx)
	for i := range runsRes.WorkflowRuns {
		jobsUrl := runsRes.WorkflowRuns[i].JobsUrl
		//fmt.Println(jobsUrl)
		//get each job data and unmarshall
		getJobsData(jobsUrl)
		insertJobData(clientDB, ctx)
	}
	genChart()
	disconnectDB()
}

func getWorkflowRunsData() {
	// Get the jobsUrl for the latest run
	url := "https://api.github.com/repos/mathworks/ci-configuration-examples/actions/runs?per_page=100&per=1&branch=hourly"
	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		panic(err)
	}

	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Add("Authorization", "token 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")
	res, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	// Error out if not a successful respond
	if res.StatusCode != 200 {
		fmt.Println("Received non-200 status code:", res.StatusCode)
	}
	defer res.Body.Close()

	err = json.NewDecoder(res.Body).Decode(&runsRes)
	if err != nil {
		panic(err)
	}
}

func getJobsData(jobsUrl string) {

	client := &http.Client{}
	req, err := http.NewRequest("GET", jobsUrl, nil)
	if err != nil {
		panic(err)
	}

	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Add("Authorization", "token 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")
	res, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	// Error out if not a successful respond
	if res.StatusCode != 200 {
		fmt.Println("Received non-200 status code:", res.StatusCode)
	}
	defer res.Body.Close()

	// Decode the JSON response
	err = json.NewDecoder(res.Body).Decode(&jobsRes)
	if err != nil {
		panic(err)
	}
	fmt.Println(jobsRes.TotalCount)

	for i := range jobsRes.Jobs {
		job := &jobsRes.Jobs[i]
		fmt.Println("=================================")
		fmt.Println("Job Name: ", job.Name)
		//fmt.Println("Job StartTime: ", job.StartTime)
		// Calculate job runtime
		runTime := getRuntime(job.StartTime, job.CompletedTime)
		job.RunTime = runTime
		//fmt.Println("Job Runtime: ", job.RunTime)

		for j := range job.Steps {
			step := &job.Steps[j]
			fmt.Println("=============================")
			fmt.Println("  Step Name: ", step.Name)
			//fmt.Println("  Step Status: ", step.Status)
			runtime := getRuntime(step.StartTime, step.CompletedTime)
			step.RunTime = runtime
			fmt.Println("  Step Runtime: ", step.RunTime)
		}
	}

}

func getRuntime(startedAt string, completedAt string) int64 {
	//Caulculate run duration time
	startedAtTime, err := time.Parse(time.RFC3339, startedAt)
	if err != nil {
		log.Fatal(err)
	}
	completedAtTime, err := time.Parse(time.RFC3339, completedAt)
	if err != nil {
		log.Fatal(err)
	}
	runtime := int64(completedAtTime.Sub(startedAtTime) / time.Second)
	return runtime
}

func connectDB() (*mongo.Client, context.Context) {
	// Create a MongoDB client, configure the client to use the URL
	// Configure a client with authentication
	// credential := options.Credential{
	// 	Username: "yitingw",
	// 	Password: "123456",
	// }
	var err error
	clientDB, err = mongo.NewClient(options.Client().ApplyURI("mongodb://yitingw:123456@labspdbg00ah.mathworks.com/setupMatlabPerfDB"))
	//client, err := mongo.NewClient(options.Client().ApplyURI("mongodb://labspdbg00ah.mathworks.com/").SetAuth(credential))
	if err != nil {
		log.Fatal(err)
	}
	ctx, _ := context.WithTimeout(context.Background(), 30*time.Second)
	err = clientDB.Connect(ctx)
	if err != nil {
		log.Fatal(err)
	}

	// Check if connect successfully
	err = clientDB.Ping(ctx, readpref.Primary())
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Connected to MongoDB successfully!")

	return clientDB, ctx
}

func disconnectDB() {
	if clientDB == nil {
		return
	}
	err := clientDB.Disconnect(context.TODO())
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Connection to MongoDB closed.")
}

func insertJobData(clientDB *mongo.Client, ctx context.Context) {

	//Insert latest data
	db := clientDB.Database(dbName)
	collection := db.Collection("jobs")
	for _, job := range jobsRes.Jobs {
		_, err := collection.InsertOne(ctx, bson.M{
			"id":         job.ID,
			"name":       job.Name,
			"label":      job.Labels,
			"status":     job.Status,
			"conclusion": job.Conclusion,
			"runtime":    job.RunTime,
			"steps":      job.Steps,
		})
		if err != nil {
			log.Fatal(err)
		}
	}
	//fmt.Println("Data saved to MongoDB")

}

func deleteOldData(clientDB *mongo.Client, ctx context.Context) {

	db := clientDB.Database(dbName)
	allCollections, err := db.ListCollectionNames(ctx, bson.D{})
	if err != nil {
		log.Fatal(err.Error())
	}

	if len(allCollections) >= 1 {
		jobsCollection := db.Collection("jobs")
		if err = jobsCollection.Drop(ctx); err != nil {
			log.Fatal(err)
		}
		fmt.Println("jobs collection dropped...")
	} else {
		fmt.Println("Database is empty...")

	}

}

//var jobNames = []string{"build-v1 (windows-2022)", "build-v1 (ubuntu-22.04)", "build-v1 (macos-12)",
//	"build-v2-beta (windows-2022)", "build-v2-beta (ubuntu-22.04)", "build-v2-beta (macos-12)"}

func logRequest(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s %s\n", r.RemoteAddr, r.Method, r.URL)
		handler.ServeHTTP(w, r)
	})
}

func genChart() {
	page := components.NewPage()
	page.AddCharts(
		//Add the Historical Failure Rate
		chart.BarTooltip(clientDB),
	)
	f, err := os.Create("charts/html/Performance.html")
	if err != nil {
		panic(err)
	}
	page.Render(io.MultiWriter(f))

	fs := http.FileServer(http.Dir("charts/html"))
	log.Println("running server at http://localhost:3001")
	log.Fatal(http.ListenAndServe("localhost:3001", logRequest(fs)))
}
