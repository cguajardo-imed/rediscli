package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"

	redis "github.com/redis/go-redis/v9"
)

var (
	ctx    context.Context
	client *redis.Client
)

func main() {
	host := os.Getenv("REDIS_HOST")
	password := os.Getenv("REDIS_PASSWORD")

	port, ok := os.LookupEnv("REDIS_PORT")
	if !ok {
		port = "6379"
	}

	dbStr, ok := os.LookupEnv("REDIS_DB")
	if !ok {
		dbStr = "0"
	}

	db, err := strconv.Atoi(dbStr)
	if err != nil {
		log.Println("error", err.Error())
		panic(err)
	}

	ctx = context.Background()
	client = redis.NewClient(&redis.Options{
		Addr:         fmt.Sprintf("%s:%s", host, port),
		Password:     password,
		DB:           db,
		PoolSize:     200,
		MinIdleConns: 50,
	})

	// Check the connection
	if !ConnectionStatus() {
		log.Println("error", "Error trying to connect with Redis")
		os.Exit(1)
	}
	log.Println("success", "Redis connection initialized on "+host+":"+port)

	// Make sure query command is provided
	if len(os.Args) < 2 {
		fmt.Println("Usage: rediscli <redis query>")
		os.Exit(1)
	}

	// Construct the command and arguments
	cmdArgs := os.Args[1:]

	// Convert string slice to interface slice
	args := make([]any, len(cmdArgs))
	for i, v := range cmdArgs {
		args[i] = v
	}

	// Send command to Redis using Do
	res, err := client.Do(ctx, args...).Result()
	if err != nil {
		fmt.Printf("Redis command error: %v\n", err)
		os.Exit(1)
	}

	// Print the result based on its type for better output
	switch val := res.(type) {
	case string:
		fmt.Println(val)
	case []byte:
		fmt.Println(string(val))
	case []any:
		for i, v := range val {
			fmt.Printf("%d) %v\n", i+1, v)
		}
	default:
		fmt.Printf("%v\n", val)
	}
}

func ConnectionStatus() bool {
	redisPingResponse := client.Ping(ctx)
	log.Println(client.Conn().String())
	if redisPingResponse.Val() != "PONG" {
		errorString, err := redisPingResponse.Result()
		if err != nil {
			log.Println("error", "Error trying to connect with Redis: "+err.Error())
		} else {
			log.Println("error", "Error trying to connect with Redis: "+errorString)
		}
		return false
	}
	return true
}
