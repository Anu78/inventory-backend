package main

import (
	"fmt"
	"time"
)

func updateList(){
	// update the grocery list here with newly low quantities
}


func Recurring() {
	// Define the time intervals for function execution
	interval := 12 * time.Hour // Function will run every hour

	// Calculate the duration until the next scheduled execution
	now := time.Now()
	nextExecution := now.Truncate(interval).Add(interval)
	if nextExecution.Before(now) {
		nextExecution = nextExecution.Add(interval)
	}
	durationUntilNextExecution := nextExecution.Sub(now)

	// Schedule the function execution using a ticker
	ticker := time.NewTicker(durationUntilNextExecution)
	defer ticker.Stop()

	for range ticker.C {
		updateList()
		fmt.Println("Function executed at:", time.Now())
	}
}
