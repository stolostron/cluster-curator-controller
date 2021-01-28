// Copyright (c) 2020 Red Hat, Inc.
package utils

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	hivev1 "github.com/openshift/hive/pkg/apis/hive/v1"
	hiveclient "github.com/openshift/hive/pkg/client/clientset/versioned"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// Simple error function
func CheckError(err error) {
	if err != nil {
		fmt.Print("\n\n")
		log.Panicln(err.Error())
	}
}

// Use to apply overrides for strings
func OverrideStringField(field *string, override string, desc string) {
	if override == "" {
		log.Println("Overriding " + desc + " \"" + *field + "\" X (NOT PROVIDED)")
	} else if *field != override {
		*field = override
		log.Println("Overriding " + desc + " \"" + *field + "\" ✓")
	} else {
		log.Println("Overriding " + desc + " \"" + *field + "\" X (ALREADY EQUAL)")
	}
}

// Use to apply overrides for int64
func OverrideInt64Field(field *int64, override string, desc string) {
	if override == "" {
		log.Println("Overriding " + desc + " \"" + strconv.FormatInt(*field, 10) + "\" X (NOT PROVIDED)")
	} else {
		overrideInt, err := strconv.ParseInt(override, 10, 64)
		CheckError(err)
		if *field != overrideInt {
			*field = overrideInt
			log.Println("Overriding " + desc + " \"" + strconv.FormatInt(*field, 10) + "\" ✓")
		} else {
			log.Println("Overriding " + desc + " \"" + strconv.FormatInt(*field, 10) + "\" X (ALREADY EQUAL)")
		}
	}
}

// Use to apply overrides for int / int32
func OverrideIntField(field *int, override string, desc string) {
	var wideField int64 = int64(*field)
	OverrideInt64Field(&wideField, override, desc)
	*field = int(wideField)
}

func MonitorDeployStatus(config *rest.Config, clusterName string) {
	hiveset, err := hiveclient.NewForConfig(config)
	CheckError(err)
	kubeset, err := kubernetes.NewForConfig(config)
	CheckError(err)
	var cluster *hivev1.ClusterDeployment
	log.Print("Checking ClusterDeployment status details")
	i := 0
	for i < 30 { // 5min wait
		i++
		// Refresh the clusterDeployment resource
		cluster, err = hiveset.HiveV1().ClusterDeployments(clusterName).Get(context.TODO(), clusterName, v1.GetOptions{})
		CheckError(err)
		if len(cluster.Status.Conditions) == 0 && cluster.Status.ProvisionRef != nil && cluster.Status.ProvisionRef.Name != "" {
			log.Println("Found ClusterDeployment status details ✓")
			jobName := cluster.Status.ProvisionRef.Name + "-provision"
			jobPath := clusterName + "/" + jobName
			log.Println("Checking for provisioning job " + jobPath)
			newJob, err := kubeset.BatchV1().Jobs(clusterName).Get(context.TODO(), jobName, v1.GetOptions{})
			// If the job is missing follow the main loop 5min timeout
			if err != nil && strings.Contains(err.Error(), " not found") {
				time.Sleep(10 * time.Second) //10s
				continue
			}
			CheckError(err)
			log.Print("Found job " + jobPath + " ✓ Start monitoring: ")
			elapsedTime := 0
			// Wait while the job is running
			log.Println("Wait for the provisioning job in Hive to complete")
			for newJob.Status.Active == 1 {
				if elapsedTime%6 == 0 {
					log.Print("Job: " + jobPath + " - " + strconv.Itoa(elapsedTime/6) + "min")
				}
				time.Sleep(10 * time.Second) //10s
				elapsedTime++
				newJob, err = kubeset.BatchV1().Jobs(clusterName).Get(context.TODO(), jobName, v1.GetOptions{})
				CheckError(err)
			}
			// If succeeded = 0 then we did not finish
			if newJob.Status.Succeeded == 0 {
				cluster, err = hiveset.HiveV1().ClusterDeployments(clusterName).Get(context.TODO(), clusterName, v1.GetOptions{})
				log.Println(cluster.Status.Conditions)
				log.Fatalln(errors.New("Provisioning job \"" + jobPath + "\" failed"))
			}
			log.Println("The provisioning job in Hive completed ✓")
			// Check if we're done
		} else if cluster.Status.WebConsoleURL != "" {
			log.Println("Provisioning succeeded ✓")
			break
			// Detect that we've failed
		} else {
			log.Print("Attempt: " + strconv.Itoa(i) + "/30, pause 10sec")
			time.Sleep(10 * time.Second) //10s
			if len(cluster.Status.Conditions) > 0 && *cluster.Spec.InstallAttemptsLimit != 0 {
				log.Println(cluster.Status.Conditions)
				log.Fatalln(errors.New("Failure detected"))
			} else if i == 19 {
				log.Fatalln(errors.New("Timed out waiting for job"))
			}
		}
	}
}
