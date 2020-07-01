package main

import (
	"context"
	// "golang.org/x/oauth2/google"
	"encoding/json"
	"fmt"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/recommender/v1"
	"log"
	"strings"
	"time"
)

func GetRecommendations(service *recommender.Service, projectId string, location string, recommenderId string) []*recommender.GoogleCloudRecommenderV1Recommendation {
	recService := recommender.NewProjectsLocationsRecommendersRecommendationsService(service)
	listCall := recService.List(fmt.Sprintf("projects/%s/locations/%s/recommenders/%s", projectId, location, recommenderId))
	var recommendations []*recommender.GoogleCloudRecommenderV1Recommendation
	addRecommendations := func(response *recommender.GoogleCloudRecommenderV1ListRecommendationsResponse) error {
		recommendations = append(recommendations, response.Recommendations...)
		return nil
	}
	ctx := context.Background()
	listCall.Pages(ctx, addRecommendations)
	return recommendations
}

func PrintRecommendations(recommendations []*recommender.GoogleCloudRecommenderV1Recommendation) {
	for _, recommendation := range recommendations {
		json, err := json.MarshalIndent(recommendation, "", "    ")
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(string(json))
	}
}

func IsStopped(instancesService *compute.InstancesService, project string, zone string, instance string) (error, bool) {
	vmInstance, err := instancesService.Get(project, zone, instance).Do()
	if err != nil {
		return err, false
	}
	return nil, vmInstance.Status == "TERMINATED"
}

func ApplyRecommendation(service *compute.Service, recommendation *GoogleCloudRecommenderV1Recommendation) error {
  operationsGroups := recommendation.Content.OperationGroups
  for group := range operationsGroups {
    for operation := range group {
      if strings.ToLower(operation.Action) == "replace" {
        if operation.Path == "/machineType" {
          // TODO: extract project, zone, instance from operation.Resource
          var project, zone, instance, machineType string
          machineTypes := "/machineTypes/"
          machineType := operation.Value[strings.Index(operation.Value, machineTypes) + len(machineTypes):]
          ChangeMachineType(service, project, zone, instance, machineType)
        }
      }
    }
  }
  return nil
}

func ChangeMachineType(service *compute.Service, project string, zone string, instance string, machineType string) error {
	instancesService := compute.NewInstancesService(service)
	_, err := instancesService.Stop(project, zone, instance).Do()
	if err != nil {
		return err
	}
	for {
		time.Sleep(time.Second)
		err, stopped := IsStopped(instancesService, project, zone, instance)
		if err != nil {
			return err
		}
		if stopped {
			break
		}
	}
	machineType = fmt.Sprintf("zones/%s/machineTypes/%s", zone, machineType)
	request := &compute.InstancesSetMachineTypeRequest{MachineType: machineType}
	_, err = instancesService.SetMachineType(project, zone, instance, request).Do()
	return err
}

func main() {
	ctx := context.Background()
	service, err := recommender.NewService(ctx)
	if err != nil {
		log.Fatal(err)
	}
	myProject := "rightsizer-test"
	defaultRecommender := "google.compute.instance.MachineTypeRecommender"
	recommendations := GetRecommendations(service, myProject, "us-central1-c", defaultRecommender)
	PrintRecommendations(recommendations)

	ctx = context.Background()
	computeService, err := compute.NewService(ctx)
	if err != nil {
		log.Fatal(err)
	}
	err = ChangeMachineType(computeService, myProject, "us-central1-a", "vkovalova-instance-memory", "e2-micro")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("machine type changed")
}
