package chart

import (
	"context"
	"log"

	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/opts"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	verNums   = []string{"v1", "v2-beta"}
	platforms = []string{"ubuntu-22.04", "macos-12", "windows-2022"}
	dbName    = "setupMatlabPerfDB"
)

func getFailureRate(clientDB *mongo.Client, verNum string, os string) float32 {

	coll := clientDB.Database(dbName).Collection("jobs")
	filter := bson.D{
		{"$and",
			bson.A{
				//select data
				bson.D{{"conclusion", "failure"}},
				bson.D{{"name", bson.D{{"$regex", verNum}}}},
				bson.D{{"name", bson.D{{"$regex", os}}}},
			}},
	}
	failureCount, err := coll.CountDocuments(context.TODO(), filter)
	if err != nil {
		log.Fatal(err)
	}
	opts := options.Count().SetHint("_id_")
	totalCount, err := coll.CountDocuments(context.TODO(), bson.D{}, opts)
	if err != nil {
		log.Fatal(err)
	}

	failureRate := float32(failureCount) / float32(totalCount) * 100

	return failureRate
}

func BarTooltip(clientDB *mongo.Client) *charts.Bar {

	bar := charts.NewBar()
	bar.SetGlobalOptions(
		charts.WithTitleOpts(opts.Title{Title: "Historical Failure Rate"}),
		charts.WithTooltipOpts(opts.Tooltip{Show: true}),
		charts.WithLegendOpts(opts.Legend{Show: true, Right: "80px"}),
	)

	failureRatesMap := make(map[string][]opts.BarData)

	for _, os := range platforms {
		failureRateOs := make([]opts.BarData, 0)
		for _, verNum := range verNums {
			failRate := getFailureRate(clientDB, verNum, os)
			failureRateOs = append(failureRateOs, opts.BarData{Value: failRate})
		}
		failureRatesMap[os] = failureRateOs
		bar.AddSeries(os, failureRateOs)
	}

	return bar
}
